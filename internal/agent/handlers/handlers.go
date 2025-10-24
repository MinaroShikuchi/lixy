package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

func HealthCheckHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check Docker availability
	cmd := exec.Command("docker", "--version")
	dockerErr := cmd.Run()

	// Check Docker Compose availability
	composeCmd := exec.Command("docker", "compose", "version")
	composeErr := composeCmd.Run()

	// Determine health status
	healthy := dockerErr == nil && composeErr == nil

	// Build appropriate response
	status := struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message,omitempty"`
	}{
		Healthy: healthy,
	}

	if !healthy {
		if dockerErr != nil {
			status.Message = "Docker engine unavailable"
		} else {
			status.Message = "Docker Compose unavailable"
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if !healthy {
		w.WriteHeader(http.StatusOK) // Still return 200 OK to allow reading the message
	}

	json.NewEncoder(w).Encode(status)

	// 	json.NewEncoder(w).Encode(map[string]string{
	// 	"status":  "healthy",
	// 	"version": "1.0.0",
	// })
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]interface{}{
		"success": false,
		"message": message,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// deployHandler handles legacy deployment requests
func DeployHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	switch r.Method {
	case http.MethodPost:
		postDeploymentHandler(w, r, logger)
	case http.MethodDelete:
		deleteDeploymentHandler(w, r, logger)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func postDeploymentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {

	// Parse the deployment request
	var deployRequest struct {
		Name        string `json:"name"`
		ComposeYAML []byte `json:"composeYAML"`
	}

	if err := json.NewDecoder(r.Body).Decode(&deployRequest); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	// Get the absolute path to your application root directory
	appRoot, err := filepath.Abs(".")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get absolute path: %v", err))
		return
	}
	// Create deployment directory using absolute path
	deployDir := filepath.Join(appRoot, "deployments", deployRequest.Name)
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create deployment directory: %v", err))
		return
	}

	// Write the compose file
	composePath := filepath.Join(deployDir, "docker-compose.yml")
	if err := ioutil.WriteFile(composePath, deployRequest.ComposeYAML, 0644); err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write compose file: %v", err))
		return
	}

	// Execute Docker Compose
	cmd := exec.Command("docker", "compose", "-f", composePath, "-p", deployRequest.Name, "up", "-d")
	cmd.Dir = deployDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Deployment failed: %v\nOutput: %s", err, output))
		return
	}

	// Return success response
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Deployment created successfully",
		"output":  string(output),
	})
}
func deleteDeploymentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	// Parse the deployment deletion request
	var deleteRequest struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	// Get the absolute path to your application root directory
	appRoot, err := filepath.Abs(".")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get absolute path: %v", err))
		return
	}
	// Define deployment directory using absolute path
	deployDir := filepath.Join(appRoot, "deployments", deleteRequest.Name)
	composePath := filepath.Join(deployDir, "docker-compose.yml")

	// Check if the deployment exists
	if _, err := os.Stat(deployDir); os.IsNotExist(err) {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Deployment %s not found", deleteRequest.Name))
		return
	}

	// Step 1: Use docker compose down to stop and remove containers
	downCmd := exec.Command("docker", "compose", "-f", composePath, "-p", deleteRequest.Name, "down", "--volumes", "--remove-orphans")
	downOutput, err := downCmd.CombinedOutput()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError,
			fmt.Sprintf("Failed to stop deployment: %v\nOutput: %s", err, downOutput))
		return
	}

	// Step 2: Remove the deployment directory
	if err := os.RemoveAll(deployDir); err != nil {
		respondWithError(w, http.StatusInternalServerError,
			fmt.Sprintf("Failed to remove deployment directory: %v", err))
		return
	}

	// Return success response
	respondWithJSON(w, http.StatusOK, map[string]string{
		"success": "true",
		"message": fmt.Sprintf("Deployment %s successfully deleted", deleteRequest.Name),
	})
}

// updateDeploymentHandler is the new endpoint specifically for deployment updates
func UpdateDeploymentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.DeploymentRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		logger.Error("Invalid update request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if string(req.ComposeYAML) == "" {
		logger.Warn("Missing composeDir in update request")
		http.Error(w, "Missing composeDir parameter", http.StatusBadRequest)
		return
	}

	logger.Info("Processing deployment update request",
		"envVarsCount", len(req.EnvVars))

	// Process the deployment update
	output, err := updateDeployment(string(req.ComposeYAML), req.EnvVars, logger)
	if err != nil {
		logger.Error("Deployment update failed", "error", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(domain.DeploymentResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update deployment: %v", err),
		})
		return
	}

	// Log detailed success information
	logger.Info("Deployment updated successfully", "outputLength", len(output))

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(domain.DeploymentResponse{
		Success: true,
		Message: "Deployment updated successfully",
		Output:  output,
	})
}

// updateDeployment performs the actual deployment update operations
func updateDeployment(composeDir string, envVars map[string]string, logger *slog.Logger) (string, error) {
	logger.Debug("Starting deployment update process",
		"composeDir", composeDir)

	// Change to the compose directory
	pullCmd := exec.Command("docker", "compose", "pull")
	pullCmd.Dir = composeDir

	// Add environment variables if specified
	env := os.Environ()
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	pullCmd.Env = env

	// Execute docker compose pull
	logger.Debug("Running docker compose pull")
	pullOutput, err := pullCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pull failed: %v - %s", err, pullOutput)
	}

	// Execute docker compose up -d
	upCmd := exec.Command("docker", "compose", "up", "-d")
	upCmd.Dir = composeDir
	upCmd.Env = env

	logger.Debug("Running docker compose up -d")
	upOutput, err := upCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("up failed: %v - %s", err, upOutput)
	}

	combinedOutput := fmt.Sprintf("Pull output:\n%s\n\nUp output:\n%s",
		pullOutput, upOutput)

	logger.Debug("Deployment commands completed successfully")
	return combinedOutput, nil
}
