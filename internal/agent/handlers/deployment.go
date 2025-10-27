package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

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

	var deployRequest domain.DeploymentRequest

	if err := json.NewDecoder(r.Body).Decode(&deployRequest); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	// if err := agent.RunDepCreateloyment(deployRequest); err != nil {
	// 	respondWithError(w, http.StatusInternalServerError,
	// 		fmt.Sprintf("Failed to run deployment: %v", err))
	// 	return
	// }

	// Return success response
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Deployment created successfully",
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

	// if err := agent.RunDeployment(deleteRequest); err != nil {
	// 	respondWithError(w, http.StatusInternalServerError,
	// 		fmt.Sprintf("Failed to run deployment: %v", err))
	// 	return
	// }

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
