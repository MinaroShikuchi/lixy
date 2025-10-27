package agent

import (
	"fmt"
	"io/ioutil"
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
	if err := ioutil.WriteFile(composePath, composeYAML, 0644); err != nil {
		return fmt.Errorf("failed to write compose file: %v", err)
	}

	// Execute Docker Compose
	cmd := exec.Command("docker", "compose", "-f", composePath, "-p", name, "up", "-d")
	cmd.Dir = deployDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deployment failed: %v\nOutput: %s", err, output)
	}

	// Step 2: Remove the deployment directory
	// if err := os.RemoveAll(deployDir); err != nil {
	// 	return fmt.Errorf("Failed to remove deployment directory: %v", err)
	// }
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
	entries, err := ioutil.ReadDir("./deployments")
	if err != nil {
		return nil, fmt.Errorf("failed to read deployments directory: %v", err)
	}

	deployments := []store.DeploymentInfo{}
	for _, entry := range entries {
		if entry.IsDir() {
			deployment := store.DeploymentInfo{
				Name: entry.Name(),
			}
			deployments = append(deployments, deployment)
		}

		deploymentID := entry.Name()
		composePath := filepath.Join("./deployments", deploymentID, "docker-compose.yml")
		cmd := exec.Command("docker", "compose", "-f", composePath, "-p", deploymentID, "ps", "-q")
		output, err := cmd.Output()
		if err != nil || len(output) == 0 {
			continue // Not running
		}

		composeData, err := ioutil.ReadFile(composePath)
		if err != nil {
			continue
		}

		deployments = append(deployments, store.DeploymentInfo{
			Name:       deploymentID,
			ComposeYML: composeData,
			// Status:     "running",
		})

	}

	return deployments, nil
}
