package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"gopkg.in/yaml.v3"
)

type DeploymentRunner struct {
	logger          *slog.Logger
	dockerConfig    string
	runtime         *ContainerRuntime
	composeCommand  []string
	pullTokenClient *PullTokenClient
}

func NewDeploymentRunner(logger *slog.Logger, runtime *ContainerRuntime, pullTokenClient *PullTokenClient) *DeploymentRunner {
	runner := &DeploymentRunner{
		logger:          logger,
		runtime:         runtime,
		pullTokenClient: pullTokenClient,
	}

	// Get the appropriate compose command
	if runtime != nil {
		runner.composeCommand = runtime.GetComposeCommand()
		logger.Info("Detected compose command", "command", strings.Join(runner.composeCommand, " "))
	} else {
		// Fallback to docker compose
		runner.composeCommand = []string{"docker", "compose"}
		logger.Warn("No runtime detected, defaulting to docker compose")
	}

	// Setup custom Docker config to avoid credential helper issues
	if err := runner.setupDockerConfig(); err != nil {
		logger.Warn("Failed to setup custom Docker config, credential helper errors may occur", "error", err)
	}

	return runner
}

// setupDockerConfig creates a custom Docker config directory without credential helpers
func (runner *DeploymentRunner) setupDockerConfig() error {
	// Create a temporary docker config directory
	configDir := filepath.Join(os.TempDir(), "lixy-docker-config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create docker config dir: %w", err)
	}

	// Create a config.json without credential helpers
	configPath := filepath.Join(configDir, "config.json")
	config := map[string]interface{}{
		"auths": map[string]interface{}{},
		// Explicitly disable credential helpers
		"credsStore": "",
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal docker config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write docker config: %w", err)
	}

	runner.dockerConfig = configDir
	runner.logger.Info("Created custom Docker config", "path", configDir)
	return nil
}

// prepareDockerCommand sets up environment for docker commands to avoid credential helper issues
func (runner *DeploymentRunner) prepareDockerCommand(cmd *exec.Cmd) {
	if runner.dockerConfig != "" {
		// Set DOCKER_CONFIG to use our custom config
		cmd.Env = append(os.Environ(), fmt.Sprintf("DOCKER_CONFIG=%s", runner.dockerConfig))
	}
}

func (runner *DeploymentRunner) Create(name string, composeYAML []byte, envVars map[string]string) error {
	appRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	deployDir := filepath.Join(appRoot, "deployments", name)
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		return fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Write the compose file
	composePath := filepath.Join(deployDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeYAML, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %v", err)
	}

	// Write .env file so docker-compose resolves ${VAR} references
	envPath := filepath.Join(deployDir, ".env")
	if len(envVars) > 0 {
		if err := runner.writeEnvFile(envPath, envVars); err != nil {
			return fmt.Errorf("failed to write .env file: %v", err)
		}
	} else {
		// Remove stale .env from a previous deploy that had secrets
		_ = os.Remove(envPath)
	}

	// Authenticate with private registries before pulling
	if err := runner.authenticatePrivateRegistries(composeYAML); err != nil {
		runner.logger.Warn("Failed to authenticate with some registries", "error", err)
	}

	// Execute Docker Compose
	args := make([]string, len(runner.composeCommand))
	copy(args, runner.composeCommand)
	args = append(args, "-f", composePath, "-p", name, "up", "-d")

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = deployDir
	runner.prepareDockerCommand(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deployment failed: %v\nOutput: %s", err, output)
	}

	return nil
}

// writeEnvFile writes key=value pairs to a .env file with 0600 permissions.
func (runner *DeploymentRunner) writeEnvFile(path string, envVars map[string]string) error {
	var sb strings.Builder
	for k, v := range envVars {
		// Escape double-quotes inside the value, then wrap in double-quotes
		escaped := strings.ReplaceAll(v, `"`, `\"`)
		sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, escaped))
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}

func (runner *DeploymentRunner) Update(deployment domain.DeploymentRequest) error {
	return runner.Create(deployment.Name, deployment.ComposeYAML, deployment.EnvVars)
}

func (runner *DeploymentRunner) Delete(name string) error {
	// Get the absolute path to your application root directory
	appRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	// Construct deployment directory path
	deployDir := filepath.Join(appRoot, "deployments", name)
	composePath := filepath.Join(deployDir, "docker-compose.yml")

	// Execute Docker Compose down
	args := make([]string, len(runner.composeCommand))
	copy(args, runner.composeCommand)
	args = append(args, "-f", composePath, "-p", name, "down", "--volumes")
	cmd := exec.Command(args[0], args[1:]...)
	runner.prepareDockerCommand(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop deployment: %v", err)
	}

	// Remove the deployment directory
	if err := os.RemoveAll(deployDir); err != nil {
		return fmt.Errorf("failed to remove deployment directory: %v", err)
	}

	return nil
}

func (runner *DeploymentRunner) ListRunning() ([]domain.DeploymentInfo, error) {
	// List all deployments from the deployment service
	entries, err := os.ReadDir("./deployments")
	if err != nil {
		return nil, fmt.Errorf("failed to read deployments directory: %v", err)
	}

	runner.logger.Info("Scanning deployments directory", "path", "./deployments", "entries", len(entries))
	deployments := []domain.DeploymentInfo{}

	for _, entry := range entries {
		// Debug log

		runner.logger.Info("Found deployment entry", "name", entry.Name(), "isDir", entry.IsDir())

		if !entry.IsDir() {
			continue
		}

		deploymentName := entry.Name()
		composePath := filepath.Join("./deployments", deploymentName, "docker-compose.yml")

		// If compose file doesn't exist, treat as not running and append basic info
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			deployments = append(deployments, domain.DeploymentInfo{
				Name:   deploymentName,
				Status: "stopped",
			})
			continue
		}

		args := make([]string, len(runner.composeCommand))
		copy(args, runner.composeCommand)
		args = append(args, "-f", composePath, "-p", deploymentName, "ps", "-q")
		cmd := exec.Command(args[0], args[1:]...)
		runner.prepareDockerCommand(cmd)
		runner.logger.Info("Checking deployment", "name", deploymentName, "cmd", cmd.String())
		output, err := cmd.Output()
		if err != nil || len(output) == 0 {
			runner.logger.Info("Deployment not running", "name", deploymentName, "error", err, "output", string(output))
			// Append as not running
			deployments = append(deployments, domain.DeploymentInfo{
				Name:   deploymentName,
				Status: "stopped",
			})
			continue // Not running
		}
		runner.logger.Info("Found running containers", "name", deploymentName, "output", string(output))

		composeData, err := os.ReadFile(composePath)
		if err != nil {
			// If compose cannot be read, still report as running with minimal info
			deployments = append(deployments, domain.DeploymentInfo{
				Name:   deploymentName,
				Status: "running",
			})
			continue
		}
		deployments = append(deployments, domain.DeploymentInfo{
			Name:        deploymentName,
			ComposeYAML: composeData,
			Status:      "running",
		})
	}

	return deployments, nil
}

// authenticatePrivateRegistries parses the compose file and authenticates with private registries
func (runner *DeploymentRunner) authenticatePrivateRegistries(composeYAML []byte) error {
	if runner.pullTokenClient == nil {
		runner.logger.Debug("No pull token client configured, skipping registry authentication")
		return nil
	}

	if runner.runtime == nil {
		return fmt.Errorf("no container runtime available")
	}

	// Parse compose file to extract images
	images, err := runner.extractImagesFromCompose(composeYAML)
	if err != nil {
		return fmt.Errorf("failed to extract images from compose file: %w", err)
	}

	runner.logger.Info("Found images in compose file", "count", len(images), "images", images)

	// Authenticate with each private registry
	registries := make(map[string]bool)
	for _, image := range images {
		isPrivate, registry := IsPrivateRegistry(image)
		if !isPrivate {
			runner.logger.Debug("Skipping public image", "image", image)
			continue
		}

		// Skip if we've already authenticated with this registry
		if registries[registry] {
			continue
		}
		registries[registry] = true

		runner.logger.Info("Authenticating with private registry", "registry", registry, "image", image)

		// Get pull token and username from controller
		token, username, err := runner.pullTokenClient.GetPullTokenWithUsername(registry)
		if err != nil {
			runner.logger.Error("Failed to get pull token", "registry", registry, "error", err)
			continue
		}

		// Authenticate with the registry using the token as password
		// Use the same docker config directory that docker-compose will use
		if err := runner.runtime.AuthenticateRegistryWithConfig(registry, username, token, runner.dockerConfig); err != nil {
			runner.logger.Error("Failed to authenticate with registry", "registry", registry, "error", err)
			continue
		}
	}

	return nil
}

// extractImagesFromCompose parses a docker-compose.yml and extracts all image references
func (runner *DeploymentRunner) extractImagesFromCompose(composeYAML []byte) ([]string, error) {
	var compose map[string]interface{}
	if err := yaml.Unmarshal(composeYAML, &compose); err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	images := []string{}

	// Extract images from services
	services, ok := compose["services"].(map[string]interface{})
	if !ok {
		return images, nil
	}

	for serviceName, serviceConfig := range services {
		config, ok := serviceConfig.(map[string]interface{})
		if !ok {
			continue
		}

		image, ok := config["image"].(string)
		if !ok {
			continue
		}

		runner.logger.Debug("Found image in compose", "service", serviceName, "image", image)
		images = append(images, image)
	}

	return images, nil
}
