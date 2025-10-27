package handlers

import (
	"encoding/json"
	"log"
	"net/http"

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

	deployments, err := dh.deploymentService.ListAllDeployments()

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
