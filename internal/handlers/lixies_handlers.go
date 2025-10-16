package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
)

// DeploymentRequest represents an incoming deployment update request
type DeploymentRequest struct {
	Action     string            `json:"action"`
	ComposeDir string            `json:"composeDir"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
}

// DeploymentResponse represents the response to a deployment request
type DeploymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

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

// deployHandler handles legacy deployment requests
func DeployHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeploymentRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	logger.Info("Deployment request received",
		"action", req.Action,
		"composeDir", req.ComposeDir)

	switch req.Action {
	case "update":
		output, err := updateDeployment(req.ComposeDir, req.EnvVars, logger)
		if err != nil {
			logger.Error("Failed to update deployment",
				"error", err,
				"composeDir", req.ComposeDir)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DeploymentResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to update deployment: %v", err),
			})
			return
		}

		// Success response
		logger.Info("Deployment updated successfully", "composeDir", req.ComposeDir)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DeploymentResponse{
			Success: true,
			Message: "Deployment updated successfully",
			Output:  output,
		})

	default:
		logger.Warn("Unknown action requested", "action", req.Action)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeploymentResponse{
			Success: false,
			Message: "Unknown action",
		})
	}
}

// updateDeploymentHandler is the new endpoint specifically for deployment updates
func UpdateDeploymentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeploymentRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		logger.Error("Invalid update request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ComposeDir == "" {
		logger.Warn("Missing composeDir in update request")
		http.Error(w, "Missing composeDir parameter", http.StatusBadRequest)
		return
	}

	logger.Info("Processing deployment update request",
		"composeDir", req.ComposeDir,
		"envVarsCount", len(req.EnvVars))

	// Process the deployment update
	output, err := updateDeployment(req.ComposeDir, req.EnvVars, logger)
	if err != nil {
		logger.Error("Deployment update failed",
			"error", err,
			"composeDir", req.ComposeDir)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeploymentResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update deployment: %v", err),
		})
		return
	}

	// Log detailed success information
	logger.Info("Deployment updated successfully",
		"composeDir", req.ComposeDir,
		"outputLength", len(output))

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(DeploymentResponse{
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
