package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

// HarborHandlers handles Harbor registry configuration, browsing, and mapping management.
type HarborHandlers struct {
	logger        *slog.Logger
	harborService *services.HarborService
}

// NewHarborHandlers creates new HarborHandlers.
func NewHarborHandlers(logger *slog.Logger, harborService *services.HarborService) *HarborHandlers {
	return &HarborHandlers{
		logger:        logger,
		harborService: harborService,
	}
}

// HandleConfig handles GET (read config) and POST (set config) for /api/harbor/config.
func (h *HarborHandlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		harborURL, username, configured := h.harborService.GetConfig()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"configured": configured,
			"config": domain.HarborConfigResponse{
				URL:      harborURL,
				Username: username,
			},
		})

	case http.MethodPost:
		var req domain.HarborConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "invalid request format"})
			return
		}
		if req.URL == "" || req.Username == "" || req.Password == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "url, username, and password are required"})
			return
		}
		if err := h.harborService.SetConfig(req.URL, req.Username, req.Password); err != nil {
			h.logger.Error("failed to set harbor config", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "failed to save harbor config"})
			return
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "harbor config saved"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ListProjects handles GET /api/harbor/projects.
func (h *HarborHandlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	projects, err := h.harborService.ListProjects()
	if err != nil {
		h.logger.Error("failed to list harbor projects", "error", err)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "projects": projects})
}

// ListRepositories handles GET /api/harbor/repositories?project=<name>.
func (h *HarborHandlers) ListRepositories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	project := r.URL.Query().Get("project")
	if project == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "project query parameter is required"})
		return
	}
	repos, err := h.harborService.ListRepositories(project)
	if err != nil {
		h.logger.Error("failed to list harbor repositories", "project", project, "error", err)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "repositories": repos})
}

// ListTags handles GET /api/harbor/repositories/{project}/{repo}/tags.
func (h *HarborHandlers) ListTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	project := r.PathValue("project")
	repo := r.PathValue("repo")
	if project == "" || repo == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "project and repo path parameters are required"})
		return
	}
	tags, err := h.harborService.ListTags(project, repo)
	if err != nil {
		h.logger.Error("failed to list harbor tags", "project", project, "repo", repo, "error", err)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "tags": tags})
}

// HandleMappings handles POST (create) and GET (list) for /api/harbor/mappings.
func (h *HarborHandlers) HandleMappings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		mappings, err := h.harborService.ListMappings()
		if err != nil {
			h.logger.Error("failed to list harbor mappings", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "failed to list mappings"})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "mappings": mappings})

	case http.MethodPost:
		var req domain.CreateHarborMappingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "invalid request format"})
			return
		}
		mapping, err := h.harborService.CreateMapping(req)
		if err != nil {
			h.logger.Error("failed to create harbor mapping", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "mapping": mapping})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// DeleteMapping handles DELETE /api/harbor/mappings/{name}.
func (h *HarborHandlers) DeleteMapping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: "mapping name is required"})
		return
	}
	if err := h.harborService.DeleteMapping(name); err != nil {
		h.logger.Error("failed to delete harbor mapping", "name", name, "error", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "mapping deleted"})
}

// syncRequest is the optional body for POST /api/harbor/sync.
type syncRequest struct {
	Name string `json:"name,omitempty"`
}

// Sync handles POST /api/harbor/sync. If body contains a name, syncs that mapping only.
func (h *HarborHandlers) Sync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req syncRequest
	// Ignore decode errors — body is optional
	json.NewDecoder(r.Body).Decode(&req)

	if req.Name != "" {
		result, err := h.harborService.SyncMapping(req.Name)
		if err != nil {
			h.logger.Error("failed to sync harbor mapping", "name", req.Name, "error", err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "result": result})
		return
	}

	results, err := h.harborService.SyncAll()
	if err != nil {
		h.logger.Error("failed to sync all harbor mappings", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{Success: false, Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "results": results})
}
