package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type DeploymentHandlers struct {
	deploymentService *services.DeploymentService
	logger            *slog.Logger
}

func NewDeploymentHandlers(deploymentService *services.DeploymentService, logger *slog.Logger) *DeploymentHandlers {
	return &DeploymentHandlers{
		deploymentService: deploymentService,
		logger:            logger,
	}
}

// func that handles creating a new deployment or list deployments
func (dh *DeploymentHandlers) CreateOrListDeployments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		dh.CreateDeployment(w, r)
	} else if r.Method == http.MethodGet {
		dh.ListDeploymentsHandler(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (dh *DeploymentHandlers) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateDeploymentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dh.logger.Error("Error decoding create deployment request", slog.String("error", err.Error()))
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	err := dh.deploymentService.CreateDeployment(req.Name, req.TargetLXC, req.ComposeYAML)
	if err != nil {
		dh.logger.Error("Failed to create deployment", slog.String("error", err.Error()))
		http.Error(w, "Failed to create deployment", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Deployment created successfully",
	})
}

// ListDeploymentsHandler handles the listing of all deployments
func (dh *DeploymentHandlers) ListDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	agentName, ok := r.Context().Value(domain.AgentNameKey).(string)
	if !ok {
		dh.logger.Error("Agent name not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dh.logger.Info("Listing deployments for agent", slog.String("agent", agentName))

	deployments, err := dh.deploymentService.ListAllDeployments("")
	if err != nil {
		dh.logger.Error("Failed to list deployments", slog.String("agent", agentName), slog.String("error", err.Error()))
		http.Error(w, "Failed to list deployments", http.StatusInternalServerError)
		return
	}

	// Return deployments as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deployments); err != nil {
		dh.logger.Error("Failed to encode deployments response", slog.String("agent", agentName), slog.String("error", err.Error()))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (dh *DeploymentHandlers) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// name is expected to be part of the URL path, e.g., /api/deployments/{name}
	name := r.URL.Path[len("/api/deployments/"):]

	var statusUpdate domain.DeploymentStatusUpdate

	if err := json.NewDecoder(r.Body).Decode(&statusUpdate); err != nil {
		dh.logger.Error("Error decoding deployment status update", slog.String("deployment", name), slog.String("error", err.Error()))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := dh.deploymentService.UpdateDeploymentStatus(name, statusUpdate.Status)
	if err != nil {
		dh.logger.Error("Failed to update deployment status", slog.String("deployment", name), slog.String("error", err.Error()))
		http.Error(w, "Failed to update deployment status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
