package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MinaroShikuchi/lixy/internal/services"
)

// GitOpsHandlers handles GitOps-related HTTP requests
type GitOpsHandlers struct {
	logger           *slog.Logger
	gitService       *services.GitService
	gitopsReconciler *services.GitOpsReconciler
}

// NewGitOpsHandlers creates a new GitOps handlers instance
func NewGitOpsHandlers(
	logger *slog.Logger,
	gitService *services.GitService,
	gitopsReconciler *services.GitOpsReconciler,
) *GitOpsHandlers {
	return &GitOpsHandlers{
		logger:           logger,
		gitService:       gitService,
		gitopsReconciler: gitopsReconciler,
	}
}

// AddRepositoryRequest represents a request to add a Git repository
type AddRepositoryRequest struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Token  string `json:"token,omitempty"`
}

// ReconcileRequest represents a request to reconcile a repository
type ReconcileRequest struct {
	RepositoryName string `json:"repository_name"`
	TargetAgent    string `json:"target_agent"`
	GithubToken    string `json:"github_token,omitempty"`
}

// SyncRequest represents a request to sync a repository
type SyncRequest struct {
	RepositoryName string `json:"repository_name"`
}

// HandleRepositories handles GET (list), POST (add), and DELETE (remove) for repositories
func (h *GitOpsHandlers) HandleRepositories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListRepositories(w, r)
	case http.MethodPost:
		h.AddRepository(w, r)
	case http.MethodDelete:
		h.RemoveRepository(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// AddRepository adds a new Git repository to monitor
func (h *GitOpsHandlers) AddRepository(w http.ResponseWriter, r *http.Request) {
	var req AddRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode add repository request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.URL == "" {
		http.Error(w, "Name and URL are required", http.StatusBadRequest)
		return
	}

	// Default branch to main if not specified
	if req.Branch == "" {
		req.Branch = "main"
	}

	// Add repository to Git service
	if err := h.gitService.AddRepository(req.Name, req.URL, req.Branch, req.Token); err != nil {
		h.logger.Error("Failed to add repository", "error", err)
		http.Error(w, "Failed to add repository", http.StatusInternalServerError)
		return
	}

	// Clone or pull the repository
	if _, err := h.gitService.CloneOrPull(req.Name); err != nil {
		h.logger.Error("Failed to clone repository", "error", err)
		http.Error(w, "Failed to clone repository", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Added repository", "name", req.Name, "url", req.URL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Repository added successfully",
		"name":    req.Name,
	})
}

// ListRepositories lists all monitored repositories
func (h *GitOpsHandlers) ListRepositories(w http.ResponseWriter, r *http.Request) {
	repos := h.gitService.ListRepositories()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"repositories": repos,
		"count":        len(repos),
	})
}

// DeleteRepository removes a repository from monitoring
func (h *GitOpsHandlers) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract repository name from path parameter
	// Expected path: /api/gitops/repositories/{name}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/gitops/repositories/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}
	repositoryName := pathParts[0]

	// Parse query parameter for deleteLocal (default to true)
	deleteLocal := true
	if r.URL.Query().Get("delete_local") == "false" {
		deleteLocal = false
	}

	// Delete the repository
	if err := h.gitService.DeleteRepository(repositoryName, deleteLocal); err != nil {
		h.logger.Error("Failed to delete repository", "name", repositoryName, "error", err)
		http.Error(w, "Failed to delete repository", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Deleted repository", "name", repositoryName, "delete_local", deleteLocal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Repository deleted successfully",
	})
}

// GetRepositoryEnvironments returns the environments available in a repository
func (h *GitOpsHandlers) GetRepositoryEnvironments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract repository name from path parameter
	// Expected path: /api/gitops/repositories/{name}/environments
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/gitops/repositories/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}
	repositoryName := pathParts[0]

	// Get environments from the repository
	environments, err := h.gitService.ListEnvironments(repositoryName)
	if err != nil {
		h.logger.Error("Failed to list environments", "repository", repositoryName, "error", err)
		http.Error(w, "Failed to list environments", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Listed environments", "repository", repositoryName, "count", len(environments))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"environments": environments,
		"count":        len(environments),
	})
}

// SyncRepository pulls latest changes from a repository
func (h *GitOpsHandlers) SyncRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode sync request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryName == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	hasChanges, err := h.gitService.CloneOrPull(req.RepositoryName)
	if err != nil {
		h.logger.Error("Failed to sync repository", "name", req.RepositoryName, "error", err)
		http.Error(w, "Failed to sync repository", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Repository synced successfully",
		"has_changes": hasChanges,
	})
}

// ReconcileRepository reconciles deployments from a repository
func (h *GitOpsHandlers) ReconcileRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReconcileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode reconcile request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryName == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	if req.TargetAgent == "" {
		http.Error(w, "Target agent is required", http.StatusBadRequest)
		return
	}

	// Sync repository first
	if _, err := h.gitService.CloneOrPull(req.RepositoryName); err != nil {
		h.logger.Error("Failed to sync repository", "name", req.RepositoryName, "error", err)
		http.Error(w, "Failed to sync repository", http.StatusInternalServerError)
		return
	}

	// Reconcile with registry authentication if GitHub token is provided
	var err error
	if req.GithubToken != "" {
		err = h.gitopsReconciler.ReconcileWithRegistry(req.RepositoryName, req.TargetAgent, req.GithubToken)
	} else {
		err = h.gitopsReconciler.ReconcileRepository(req.RepositoryName, req.TargetAgent)
	}

	if err != nil {
		h.logger.Error("Failed to reconcile repository",
			"name", req.RepositoryName,
			"target", req.TargetAgent,
			"error", err)
		http.Error(w, "Failed to reconcile repository", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully reconciled repository",
		"name", req.RepositoryName,
		"target", req.TargetAgent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Repository reconciled successfully",
	})
}

// ReconcileEnvironmentRequest represents a request to reconcile an environment
type ReconcileEnvironmentRequest struct {
	RepositoryName string `json:"repository_name"`
	Environment    string `json:"environment"` // e.g., "dev", "staging", "production"
	GithubToken    string `json:"github_token,omitempty"`
}

// DriftDetectionRequest represents a request to detect drift
type DriftDetectionRequest struct {
	RepositoryName string `json:"repository_name"`
	Environment    string `json:"environment"`
}

// ReconcileEnvironment reconciles all services in an environment
func (h *GitOpsHandlers) ReconcileEnvironment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ReconcileEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode reconcile environment request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryName == "" || req.Environment == "" {
		http.Error(w, "Repository name and environment are required", http.StatusBadRequest)
		return
	}

	// Sync repository first
	if _, err := h.gitService.CloneOrPull(req.RepositoryName); err != nil {
		h.logger.Error("Failed to sync repository", "name", req.RepositoryName, "error", err)
		http.Error(w, "Failed to sync repository", http.StatusInternalServerError)
		return
	}

	// Reconcile the environment using the new deployment-based approach
	if err := h.gitopsReconciler.ReconcileEnvironmentDeployments(req.RepositoryName, req.Environment); err != nil {
		h.logger.Error("Failed to reconcile environment",
			"repository", req.RepositoryName,
			"environment", req.Environment,
			"error", err)
		http.Error(w, "Failed to reconcile environment", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully reconciled environment",
		"repository", req.RepositoryName,
		"environment", req.Environment)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Environment reconciled successfully",
		"environment": req.Environment,
	})
}

// DetectDrift detects configuration drift in an environment
func (h *GitOpsHandlers) DetectDrift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DriftDetectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode drift detection request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryName == "" || req.Environment == "" {
		http.Error(w, "Repository name and environment are required", http.StatusBadRequest)
		return
	}

	// Sync repository first
	if _, err := h.gitService.CloneOrPull(req.RepositoryName); err != nil {
		h.logger.Error("Failed to sync repository", "name", req.RepositoryName, "error", err)
		http.Error(w, "Failed to sync repository", http.StatusInternalServerError)
		return
	}

	// Detect drift
	driftedServices, err := h.gitopsReconciler.DetectDrift(req.RepositoryName, req.Environment)
	if err != nil {
		h.logger.Error("Failed to detect drift",
			"repository", req.RepositoryName,
			"environment", req.Environment,
			"error", err)
		http.Error(w, "Failed to detect drift", http.StatusInternalServerError)
		return
	}

	hasDrift := len(driftedServices) > 0

	h.logger.Info("Drift detection complete",
		"repository", req.RepositoryName,
		"environment", req.Environment,
		"drifted_services", len(driftedServices))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"has_drift":        hasDrift,
		"drifted_services": driftedServices,
		"count":            len(driftedServices),
	})
}

// CheckUpdatesRequest represents a request to check for updates
type CheckUpdatesRequest struct {
	RepositoryName string `json:"repository_name"`
	Environment    string `json:"environment"`
	GithubToken    string `json:"github_token,omitempty"`
}

// CheckUpdates checks for available updates for services
func (h *GitOpsHandlers) CheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckUpdatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode check updates request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryName == "" || req.Environment == "" {
		http.Error(w, "Repository name and environment are required", http.StatusBadRequest)
		return
	}

	// Sync repository first
	if _, err := h.gitService.CloneOrPull(req.RepositoryName); err != nil {
		h.logger.Error("Failed to sync repository", "name", req.RepositoryName, "error", err)
		http.Error(w, "Failed to sync repository", http.StatusInternalServerError)
		return
	}

	// Check for updates
	updates, err := h.gitopsReconciler.CheckForUpdates(req.RepositoryName, req.Environment, req.GithubToken)
	if err != nil {
		h.logger.Error("Failed to check for updates",
			"repository", req.RepositoryName,
			"environment", req.Environment,
			"error", err)
		http.Error(w, "Failed to check for updates", http.StatusInternalServerError)
		return
	}

	hasUpdates := len(updates) > 0

	h.logger.Info("Update check complete",
		"repository", req.RepositoryName,
		"environment", req.Environment,
		"updates_available", len(updates))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"has_updates": hasUpdates,
		"updates":     updates,
		"count":       len(updates),
	})
}

// RemoveRepositoryRequest represents a request to remove a Git repository
type RemoveRepositoryRequest struct {
	Name string `json:"name"`
}

// RemoveRepository removes a Git repository from monitoring
func (h *GitOpsHandlers) RemoveRepository(w http.ResponseWriter, r *http.Request) {
	var req RemoveRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	if err := h.gitService.RemoveRepository(req.Name); err != nil {
		h.logger.Error("Failed to remove repository", "error", err)
		http.Error(w, "Failed to remove repository", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Removed repository", "name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Repository removed successfully",
		"name":    req.Name,
	})
}

// UpdateRepositoryTokenRequest represents a request to update repository token
type UpdateRepositoryTokenRequest struct {
	Token string `json:"token"`
}

// UpdateRepositoryToken updates the authentication token for a repository
func (h *GitOpsHandlers) UpdateRepositoryToken(w http.ResponseWriter, r *http.Request) {
	// Extract repository name from URL path
	repoName := r.PathValue("name")
	if repoName == "" {
		http.Error(w, "Repository name is required", http.StatusBadRequest)
		return
	}

	var req UpdateRepositoryTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.gitService.UpdateRepositoryToken(repoName, req.Token); err != nil {
		h.logger.Error("Failed to update repository token", "error", err)
		http.Error(w, "Failed to update repository token", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Updated repository token", "name", repoName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Repository token updated successfully",
		"name":    repoName,
	})
}
