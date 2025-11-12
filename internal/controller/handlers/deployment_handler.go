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
	switch r.Method {
	case http.MethodPost:
		dh.CreateDeployment(w, r)
	case http.MethodGet:
		dh.ListDeploymentsHandler(w, r)
	default:
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

// HandleDeploymentByName routes requests to the appropriate handler based on HTTP method
func (dh *DeploymentHandlers) HandleDeploymentByName(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPatch:
		dh.UpdateDeploymentStatus(w, r)
	case http.MethodDelete:
		dh.DeleteDeployment(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (dh *DeploymentHandlers) UpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
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

// DeleteDeployment handles DELETE requests to remove a deployment
func (dh *DeploymentHandlers) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	// Extract deployment name from URL path, e.g., /api/deployments/{name}
	name := r.URL.Path[len("/api/deployments/"):]
	if name == "" {
		dh.logger.Error("Deployment name not provided in URL")
		http.Error(w, "Deployment name is required", http.StatusBadRequest)
		return
	}

	dh.logger.Info("Deleting deployment", slog.String("deployment", name))

	err := dh.deploymentService.DeleteDeployment(name)
	if err != nil {
		dh.logger.Error("Failed to delete deployment", slog.String("deployment", name), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	dh.logger.Info("Deployment deleted successfully", slog.String("deployment", name))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Deployment deleted successfully",
		"name":    name,
	})
}
