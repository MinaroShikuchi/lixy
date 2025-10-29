package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type DeploymentHandlers struct {
	deploymentService *services.DeploymentService
}

func NewDeploymentHandlers(deploymentService *services.DeploymentService) *DeploymentHandlers {
	return &DeploymentHandlers{
		deploymentService: deploymentService,
	}
}

// ListDeploymentsHandler handles the listing of all deployments
func (dh *DeploymentHandlers) ListDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request for %s", r.Method, r.URL.Path)

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentName := r.Context().Value("agent_name").(string)

	deployments, err := dh.deploymentService.ListAllDeployments("")
	if err != nil {
		log.Printf("Error listing deployments for agent %s: %v", agentName, err)
		http.Error(w, "Failed to list deployments", http.StatusInternalServerError)
		return
	}

	// Return deployments as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deployments); err != nil {
		log.Printf("Error encoding deployments response for agent %s: %v", agentName, err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (dh *DeploymentHandlers) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request for %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// name is expected to be part of the URL path, e.g., /api/deployments/{name}
	name := r.URL.Path[len("/api/deployments/"):]

	var statusUpdate domain.DeploymentStatusUpdate

	if err := json.NewDecoder(r.Body).Decode(&statusUpdate); err != nil {
		log.Printf("Error decoding status update for deployment %s: %v", name, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Here you would typically update the deployment status in your store
	log.Printf("Updating deployment %s to status %s", name, statusUpdate.Status)

	err := dh.deploymentService.UpdateDeploymentStatus(name, statusUpdate.Status)
	if err != nil {
		log.Printf("Error updating deployment status for %s: %v", name, err)
		http.Error(w, "Failed to update deployment status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
