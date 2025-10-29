package client

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

type DeploymentRunner struct {
	logger *slog.Logger
}

func NewDeploymentRunner(logger *slog.Logger) *DeploymentRunner {
	return &DeploymentRunner{
		logger: logger,
	}
}

func (runner *DeploymentRunner) Create(name string, composeYAML []byte) error {
	// Get the absolute path to your application root directory
	appRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	// Create deployment directory using absolute path
	deployDir := filepath.Join(appRoot, "deployments", name)
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		return fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Write the compose file
	composePath := filepath.Join(deployDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeYAML, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %v", err)
	}

	// Execute Docker Compose
	cmd := exec.Command("docker", "compose", "-f", composePath, "-p", name, "up", "-d")
	cmd.Dir = deployDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deployment failed: %v\nOutput: %s", err, output)
	}

	return nil
}

func (runner *DeploymentRunner) Update(deployment domain.DeploymentRequest) error {
	// For simplicity, we treat update the same as create in this example
	return runner.Create(deployment.Name, deployment.ComposeYAML)
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
	cmd := exec.Command("docker", "compose", "-f", composePath, "-p", name, "down", "--volumes")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop deployment: %v", err)
	}

	// Remove the deployment directory
	if err := os.RemoveAll(deployDir); err != nil {
		return fmt.Errorf("failed to remove deployment directory: %v", err)
	}

	return nil
}

func (runner *DeploymentRunner) ListRunning() ([]store.DeploymentInfo, error) {
	// List all deployments from the deployment service
	entries, err := os.ReadDir("./deployments")
	if err != nil {
		return nil, fmt.Errorf("failed to read deployments directory: %v", err)
	}

	runner.logger.Info("Scanning deployments directory", "path", "./deployments", "entries", len(entries))
	deployments := []store.DeploymentInfo{}

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
			deployments = append(deployments, store.DeploymentInfo{
				Name:   deploymentName,
				Status: "stopped",
			})
			continue
		}

		cmd := exec.Command("docker", "compose", "-f", composePath, "-p", deploymentName, "ps", "-q")
		runner.logger.Info("Checking deployment", "name", deploymentName, "cmd", cmd.String())
		output, err := cmd.Output()
		if err != nil || len(output) == 0 {
			runner.logger.Info("Deployment not running", "name", deploymentName, "error", err, "output", string(output))
			// Append as not running
			deployments = append(deployments, store.DeploymentInfo{
				Name:   deploymentName,
				Status: "stopped",
			})
			continue // Not running
		}
		runner.logger.Info("Found running containers", "name", deploymentName, "output", string(output))

		composeData, err := os.ReadFile(composePath)
		if err != nil {
			// If compose cannot be read, still report as running with minimal info
			deployments = append(deployments, store.DeploymentInfo{
				Name:   deploymentName,
				Status: "running",
			})
			continue
		}
		deployments = append(deployments, store.DeploymentInfo{
			Name:        deploymentName,
			ComposeYAML: composeData,
			Status:      "running",
		})
	}

	return deployments, nil
}
