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
// Supports both user and agent authentication
func (dh *DeploymentHandlers) ListDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	// Check token type to determine caller
	tokenType, _ := r.Context().Value(domain.TokenTypeKey).(string)

	var caller string
	if tokenType == "user" {
		// User token - get username
		username, ok := r.Context().Value(domain.UsernameKey).(string)
		if !ok {
			dh.logger.Error("Username not found in context for user token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		caller = "user:" + username
	} else {
		// Agent token - get agent name
		agentName, ok := r.Context().Value(domain.AgentNameKey).(string)
		if !ok {
			dh.logger.Error("Agent name not found in context for agent token")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		caller = "agent:" + agentName
	}

	dh.logger.Info("Listing deployments", slog.String("caller", caller))

	deployments, err := dh.deploymentService.ListAllDeployments("")
	if err != nil {
		dh.logger.Error("Failed to list deployments", slog.String("caller", caller), slog.String("error", err.Error()))
		http.Error(w, "Failed to list deployments", http.StatusInternalServerError)
		return
	}

	result := make([]domain.DeploymentDto, 0, len(deployments))
	for _, d := range deployments {
		targets := []string{}
		if d.TargetLXC != "" {
			targets = []string{d.TargetLXC}
		}
		composeStr := string(d.ComposeYAML)
		result = append(result, domain.DeploymentDto{
			ID:           d.ID,
			Name:         d.Name,
			Status:       d.Status,
			TargetLXC:    d.TargetLXC,
			TargetAgents: targets,
			ComposeYAML:  composeStr,
			ComposeFile:  composeStr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		dh.logger.Error("Failed to encode deployments response", slog.String("caller", caller), slog.String("error", err.Error()))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleDeploymentByName routes requests to the appropriate handler based on HTTP method
func (dh *DeploymentHandlers) HandleDeploymentByName(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPatch:
		dh.UpdateDeploymentStatus(w, r)
	case http.MethodPut:
		dh.UpdateDeploymentCompose(w, r)
	case http.MethodDelete:
		dh.DeleteDeployment(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// UpdateDeploymentCompose handles PUT /api/deployments/{name} to update compose YAML
func (dh *DeploymentHandlers) UpdateDeploymentCompose(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/api/deployments/"):]
	if name == "" {
		http.Error(w, "Deployment name is required", http.StatusBadRequest)
		return
	}

	var req struct {
		ComposeYAML string `json:"compose_yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ComposeYAML == "" {
		http.Error(w, "compose_yaml is required", http.StatusBadRequest)
		return
	}

	if err := dh.deploymentService.UpdateDeployment(name, []byte(req.ComposeYAML)); err != nil {
		dh.logger.Error("Failed to update deployment compose", "deployment", name, "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Deployment compose updated",
	})
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
