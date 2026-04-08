package daemon

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DaemonConfig holds all configuration needed to install or uninstall a daemon.
type DaemonConfig struct {
	ServiceName  string            // "lixy" or "lixies"
	DisplayName  string            // "Lixy Controller" or "Lixies Agent"
	Description  string            // systemd description
	BinaryName   string            // "lixy" or "lixies"
	InstallDir   string            // "/opt/lixy" or "/opt/lixies"
	ConfigDir    string            // "/etc/lixy"
	DataDir      string            // "/var/lib/lixy" or "/var/lib/lixies"
	LogDir       string            // "/var/log/lixy" or "/var/log/lixies"
	SocketDir    string            // directory for unix socket
	User         string            // "lixy" or "lixies"
	Group        string            // "lixy" or "lixies"
	EnvVars      map[string]string // environment variables for the service
	ConfigSource string            // path to source config file (e.g., "./lixy.yaml")
	ConfigDest   string            // destination config filename (e.g., "lixy.yaml")
}

// ANSI color helpers
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func stepOK(msg string) {
	fmt.Printf("  %s✓%s %s\n", colorGreen, colorReset, msg)
}

func stepFail(msg string) {
	fmt.Printf("  %s✗%s %s\n", colorRed, colorReset, msg)
}

func stepInfo(msg string) {
	fmt.Printf("  %s→%s %s\n", colorCyan, colorReset, msg)
}

func stepWarn(msg string) {
	fmt.Printf("  %s!%s %s\n", colorYellow, colorReset, msg)
}

func header(msg string) {
	fmt.Printf("\n%s%s%s\n", colorBold, msg, colorReset)
}

// Install performs a full installation of the daemon as a systemd service.
func Install(cfg DaemonConfig) error {
	header(fmt.Sprintf("Installing %s as a systemd service", cfg.DisplayName))
	fmt.Println()

	// Step 1: Check root privileges
	if os.Geteuid() != 0 {
		stepFail("This command must be run as root (use sudo)")
		return fmt.Errorf("root privileges required: run with sudo")
	}
	stepOK("Running with root privileges")

	// Step 2: Create system user and group
	if err := createSystemUser(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to create system user: %v", err))
		return fmt.Errorf("failed to create system user: %w", err)
	}

	// Step 3: Create directories
	dirs := map[string]string{
		"install":   cfg.InstallDir,
		"config":    cfg.ConfigDir,
		"data":      cfg.DataDir,
		"log":       cfg.LogDir,
		"socket":    cfg.SocketDir,
	}
	for name, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			stepFail(fmt.Sprintf("Failed to create %s directory %s: %v", name, dir, err))
			return fmt.Errorf("failed to create %s directory: %w", name, err)
		}
		stepOK(fmt.Sprintf("Created %s directory: %s", name, dir))
	}

	// Step 4: Copy the currently running binary
	if err := copyBinary(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to copy binary: %v", err))
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Step 5: Copy config file (don't overwrite existing)
	if err := copyConfig(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to copy config: %v", err))
		return fmt.Errorf("failed to copy config: %w", err)
	}

	// Step 6: Update config file paths to absolute paths
	if err := updateConfigPaths(cfg); err != nil {
		stepWarn(fmt.Sprintf("Could not update config paths: %v", err))
		// Non-fatal — the service may still work with defaults
	}

	// Step 6b: Generate environment file with secrets
	if err := generateEnvFile(cfg); err != nil {
		stepWarn(fmt.Sprintf("Could not generate environment file: %v", err))
		// Non-fatal — user can set env vars manually
	}

	// Step 7: Set ownership
	if err := setOwnership(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to set ownership: %v", err))
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	// Step 8: Set permissions
	if err := setPermissions(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to set permissions: %v", err))
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Step 9: Generate and write systemd unit file
	if err := writeUnitFile(cfg); err != nil {
		stepFail(fmt.Sprintf("Failed to write systemd unit file: %v", err))
		return fmt.Errorf("failed to write systemd unit file: %w", err)
	}
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", cfg.ServiceName)
	stepOK(fmt.Sprintf("Created systemd unit file: %s", unitPath))

	// Step 10: Reload systemd
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		stepFail(fmt.Sprintf("Failed to reload systemd: %v", err))
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	stepOK("Reloaded systemd daemon")

	// Step 11: Enable the service
	if err := runCommand("systemctl", "enable", cfg.ServiceName); err != nil {
		stepFail(fmt.Sprintf("Failed to enable service: %v", err))
		return fmt.Errorf("failed to enable service: %w", err)
	}
	stepOK(fmt.Sprintf("Enabled %s service", cfg.ServiceName))

	// Step 12: Start the service
	if err := runCommand("systemctl", "start", cfg.ServiceName); err != nil {
		stepFail(fmt.Sprintf("Failed to start service: %v", err))
		fmt.Printf("\n  %sTip:%s Try checking the logs with: journalctl -u %s -f\n", colorYellow, colorReset, cfg.ServiceName)
		return fmt.Errorf("failed to start service: %w", err)
	}
	stepOK(fmt.Sprintf("Started %s service", cfg.ServiceName))

	// Step 13: Print success message
	printInstallSuccess(cfg)

	return nil
}

// Uninstall removes the daemon systemd service and optionally purges all data.
func Uninstall(cfg DaemonConfig, purge bool) error {
	header(fmt.Sprintf("Uninstalling %s", cfg.DisplayName))
	fmt.Println()

	// Step 1: Check root privileges
	if os.Geteuid() != 0 {
		stepFail("This command must be run as root (use sudo)")
		return fmt.Errorf("root privileges required: run with sudo")
	}
	stepOK("Running with root privileges")

	// Step 2: Stop the service (ignore errors if not running)
	if err := runCommand("systemctl", "stop", cfg.ServiceName); err != nil {
		stepWarn(fmt.Sprintf("Service %s may not be running (stop returned: %v)", cfg.ServiceName, err))
	} else {
		stepOK(fmt.Sprintf("Stopped %s service", cfg.ServiceName))
	}

	// Step 3: Disable the service (ignore errors)
	if err := runCommand("systemctl", "disable", cfg.ServiceName); err != nil {
		stepWarn(fmt.Sprintf("Service %s may not be enabled (disable returned: %v)", cfg.ServiceName, err))
	} else {
		stepOK(fmt.Sprintf("Disabled %s service", cfg.ServiceName))
	}

	// Step 4: Remove unit file
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", cfg.ServiceName)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		stepFail(fmt.Sprintf("Failed to remove unit file: %v", err))
		return fmt.Errorf("failed to remove unit file: %w", err)
	}
	stepOK(fmt.Sprintf("Removed unit file: %s", unitPath))

	// Step 5: Reload systemd
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		stepWarn(fmt.Sprintf("Failed to reload systemd: %v", err))
	} else {
		stepOK("Reloaded systemd daemon")
	}

	// Step 6: Purge data if requested
	if purge {
		header("Purging all data")
		fmt.Println()

		removeDirIfExists(cfg.InstallDir, "install directory")
		removeDirIfExists(cfg.DataDir, "data directory")
		removeDirIfExists(cfg.LogDir, "log directory")

		// Remove config dir only if it's empty or purge is explicit
		if err := os.Remove(cfg.ConfigDir); err != nil {
			if os.IsNotExist(err) {
				stepInfo(fmt.Sprintf("Config directory already removed: %s", cfg.ConfigDir))
			} else {
				// Directory not empty — try removing recursively since purge is explicit
				if err := os.RemoveAll(cfg.ConfigDir); err != nil {
					stepWarn(fmt.Sprintf("Could not remove config directory %s: %v", cfg.ConfigDir, err))
				} else {
					stepOK(fmt.Sprintf("Removed config directory: %s", cfg.ConfigDir))
				}
			}
		} else {
			stepOK(fmt.Sprintf("Removed config directory: %s", cfg.ConfigDir))
		}

		// Remove system user
		if err := runCommand("userdel", cfg.User); err != nil {
			stepWarn(fmt.Sprintf("Could not remove user %s: %v", cfg.User, err))
		} else {
			stepOK(fmt.Sprintf("Removed system user: %s", cfg.User))
		}
	}

	printUninstallSuccess(cfg, purge)

	return nil
}

// createSystemUser creates a system user and group for the service.
func createSystemUser(cfg DaemonConfig) error {
	// Check if user already exists
	if err := runCommand("id", cfg.User); err == nil {
		stepInfo(fmt.Sprintf("System user %s already exists, skipping creation", cfg.User))
		return nil
	}

	// Create system user (also creates group with same name)
	if err := runCommand("useradd", "-r", "-s", "/bin/false", "-d", cfg.InstallDir, cfg.User); err != nil {
		return fmt.Errorf("useradd failed: %w", err)
	}
	stepOK(fmt.Sprintf("Created system user and group: %s", cfg.User))
	return nil
}

// copyBinary copies the currently running executable to the install directory.
func copyBinary(cfg DaemonConfig) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	// Resolve symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}

	destPath := filepath.Join(cfg.InstallDir, cfg.BinaryName)

	src, err := os.Open(exePath)
	if err != nil {
		return fmt.Errorf("could not open source binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("could not create destination binary: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("could not copy binary: %w", err)
	}

	stepOK(fmt.Sprintf("Copied binary to %s", destPath))
	return nil
}

// copyConfig copies the config file to the config directory if it doesn't already exist.
func copyConfig(cfg DaemonConfig) error {
	destPath := filepath.Join(cfg.ConfigDir, cfg.ConfigDest)

	// Don't overwrite existing config
	if _, err := os.Stat(destPath); err == nil {
		stepInfo(fmt.Sprintf("Config file already exists at %s, skipping (won't overwrite)", destPath))
		return nil
	}

	// Check if source config exists
	if _, err := os.Stat(cfg.ConfigSource); err != nil {
		stepWarn(fmt.Sprintf("Source config %s not found, skipping config copy", cfg.ConfigSource))
		stepInfo("The service will use default configuration values")
		return nil
	}

	src, err := os.Open(cfg.ConfigSource)
	if err != nil {
		return fmt.Errorf("could not open source config: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return fmt.Errorf("could not create destination config: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("could not copy config: %w", err)
	}

	stepOK(fmt.Sprintf("Copied config to %s", destPath))
	return nil
}

// updateConfigPaths reads the installed config YAML and updates relative paths
// to use absolute paths under the daemon directories.
func updateConfigPaths(cfg DaemonConfig) error {
	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigDest)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	// Parse YAML into a generic map to preserve structure
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("could not parse config YAML: %w", err)
	}

	modified := false

	// Update database path
	if db, ok := configMap["database"].(string); ok {
		baseName := filepath.Base(db)
		newPath := filepath.Join(cfg.DataDir, baseName)
		configMap["database"] = newPath
		modified = true
		stepOK(fmt.Sprintf("Updated database path: %s", newPath))
	}

	// Update log_file path
	if logFile, ok := configMap["log_file"].(string); ok {
		baseName := filepath.Base(logFile)
		newPath := filepath.Join(cfg.LogDir, baseName)
		configMap["log_file"] = newPath
		modified = true
		stepOK(fmt.Sprintf("Updated log_file path: %s", newPath))
	}

	// Update socket_path
	if socketPath, ok := configMap["socket_path"].(string); ok {
		baseName := filepath.Base(socketPath)
		newPath := filepath.Join(cfg.SocketDir, baseName)
		configMap["socket_path"] = newPath
		modified = true
		stepOK(fmt.Sprintf("Updated socket_path: %s", newPath))
	}

	if !modified {
		stepInfo("No paths to update in config file")
		return nil
	}

	// Write back
	out, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("could not marshal updated config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0640); err != nil {
		return fmt.Errorf("could not write updated config: %w", err)
	}

	return nil
}

// generateRandomKey generates a cryptographically random 32-byte key, base64-encoded.
func generateRandomKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("could not generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// generateEnvFile creates an environment file with auto-generated secrets
// if one doesn't already exist. The file is referenced by the systemd unit
// via EnvironmentFile= directive.
func generateEnvFile(cfg DaemonConfig) error {
	envPath := filepath.Join(cfg.ConfigDir, cfg.ServiceName+".env")

	// Don't overwrite existing env file (secrets should be preserved)
	if _, err := os.Stat(envPath); err == nil {
		stepInfo(fmt.Sprintf("Environment file already exists at %s, skipping (won't overwrite)", envPath))
		return nil
	}

	var lines []string

	// Generate LIXY_JWT_SECRET if not already set in environment
	jwtSecret := os.Getenv("LIXY_JWT_SECRET")
	if jwtSecret == "" {
		var err error
		jwtSecret, err = generateRandomKey()
		if err != nil {
			return fmt.Errorf("could not generate JWT secret: %w", err)
		}
		stepOK("Generated new LIXY_JWT_SECRET")
	} else {
		stepInfo("Using LIXY_JWT_SECRET from current environment")
	}
	lines = append(lines, fmt.Sprintf("LIXY_JWT_SECRET=%s", jwtSecret))

	// Generate LIXY_ENCRYPTION_KEY if not already set in environment
	encKey := os.Getenv("LIXY_ENCRYPTION_KEY")
	if encKey == "" {
		var err error
		encKey, err = generateRandomKey()
		if err != nil {
			return fmt.Errorf("could not generate encryption key: %w", err)
		}
		stepOK("Generated new LIXY_ENCRYPTION_KEY")
	} else {
		stepInfo("Using LIXY_ENCRYPTION_KEY from current environment")
	}
	lines = append(lines, fmt.Sprintf("LIXY_ENCRYPTION_KEY=%s", encKey))

	content := strings.Join(lines, "\n") + "\n"

	// Write with restrictive permissions (only owner can read)
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("could not write environment file: %w", err)
	}

	stepOK(fmt.Sprintf("Created environment file: %s", envPath))
	return nil
}

// setOwnership sets ownership of all daemon directories to the service user.
func setOwnership(cfg DaemonConfig) error {
	dirs := []string{cfg.InstallDir, cfg.ConfigDir, cfg.DataDir, cfg.LogDir, cfg.SocketDir}
	for _, dir := range dirs {
		if err := runCommand("chown", "-R", fmt.Sprintf("%s:%s", cfg.User, cfg.Group), dir); err != nil {
			return fmt.Errorf("chown failed for %s: %w", dir, err)
		}
	}
	stepOK(fmt.Sprintf("Set ownership to %s:%s on all directories", cfg.User, cfg.Group))
	return nil
}

// setPermissions sets appropriate permissions on the binary, config, and directories.
func setPermissions(cfg DaemonConfig) error {
	// Binary: executable
	binaryPath := filepath.Join(cfg.InstallDir, cfg.BinaryName)
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("chmod failed for binary: %w", err)
	}

	// Config: readable by owner and group
	configPath := filepath.Join(cfg.ConfigDir, cfg.ConfigDest)
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Chmod(configPath, 0640); err != nil {
			return fmt.Errorf("chmod failed for config: %w", err)
		}
	}

	// Directories: 755
	dirs := []string{cfg.InstallDir, cfg.ConfigDir, cfg.DataDir, cfg.LogDir, cfg.SocketDir}
	for _, dir := range dirs {
		if err := os.Chmod(dir, 0755); err != nil {
			return fmt.Errorf("chmod failed for %s: %w", dir, err)
		}
	}

	stepOK("Set file and directory permissions")
	return nil
}

// runCommand executes a system command and returns any error.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// removeDirIfExists removes a directory tree, printing status.
func removeDirIfExists(dir, label string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		stepInfo(fmt.Sprintf("%s already removed: %s", strings.Title(label), dir))
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		stepWarn(fmt.Sprintf("Could not remove %s %s: %v", label, dir, err))
	} else {
		stepOK(fmt.Sprintf("Removed %s: %s", label, dir))
	}
}

// printInstallSuccess prints a summary after successful installation.
func printInstallSuccess(cfg DaemonConfig) {
	fmt.Println()
	fmt.Printf("%s%s════════════════════════════════════════════════════════%s\n", colorBold, colorGreen, colorReset)
	fmt.Printf("%s%s  %s installed successfully!%s\n", colorBold, colorGreen, cfg.DisplayName, colorReset)
	fmt.Printf("%s%s════════════════════════════════════════════════════════%s\n", colorBold, colorGreen, colorReset)
	fmt.Println()
	fmt.Printf("  %sService:%s       %s\n", colorBold, colorReset, cfg.ServiceName)
	fmt.Printf("  %sBinary:%s        %s/%s\n", colorBold, colorReset, cfg.InstallDir, cfg.BinaryName)
	fmt.Printf("  %sConfig:%s        %s/%s\n", colorBold, colorReset, cfg.ConfigDir, cfg.ConfigDest)
	fmt.Printf("  %sData:%s          %s\n", colorBold, colorReset, cfg.DataDir)
	fmt.Printf("  %sLogs:%s          %s\n", colorBold, colorReset, cfg.LogDir)
	fmt.Println()
	fmt.Printf("  %sUseful commands:%s\n", colorBold, colorReset)
	fmt.Printf("    sudo systemctl status %s    # Check service status\n", cfg.ServiceName)
	fmt.Printf("    sudo systemctl restart %s   # Restart the service\n", cfg.ServiceName)
	fmt.Printf("    sudo journalctl -u %s -f    # Follow systemd logs\n", cfg.ServiceName)
	fmt.Printf("    sudo tail -f %s/%s.log  # Follow application logs\n", cfg.LogDir, cfg.ServiceName)
	fmt.Println()
}

// printUninstallSuccess prints a summary after successful uninstallation.
func printUninstallSuccess(cfg DaemonConfig, purge bool) {
	fmt.Println()
	fmt.Printf("%s%s════════════════════════════════════════════════════════%s\n", colorBold, colorGreen, colorReset)
	fmt.Printf("%s%s  %s uninstalled successfully!%s\n", colorBold, colorGreen, cfg.DisplayName, colorReset)
	fmt.Printf("%s%s════════════════════════════════════════════════════════%s\n", colorBold, colorGreen, colorReset)
	fmt.Println()
	if purge {
		fmt.Printf("  All data, logs, and configuration have been removed.\n")
	} else {
		fmt.Printf("  The service has been stopped and disabled.\n")
		fmt.Printf("  Data and configuration files have been preserved:\n")
		fmt.Printf("    %sConfig:%s  %s/%s\n", colorBold, colorReset, cfg.ConfigDir, cfg.ConfigDest)
		fmt.Printf("    %sData:%s    %s\n", colorBold, colorReset, cfg.DataDir)
		fmt.Printf("    %sLogs:%s    %s\n", colorBold, colorReset, cfg.LogDir)
		fmt.Println()
		fmt.Printf("  To fully remove all data, run:\n")
		fmt.Printf("    sudo %s/%s uninstall --purge\n", cfg.InstallDir, cfg.BinaryName)
	}
	fmt.Println()
}
