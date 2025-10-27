package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
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
