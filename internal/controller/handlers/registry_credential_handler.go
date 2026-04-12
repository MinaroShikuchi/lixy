package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MinaroShikuchi/lixy/internal/store"
)

// RegistryCredentialHandlers handles registry credential management
type RegistryCredentialHandlers struct {
	logger    *slog.Logger
	credStore *store.RegistryCredentialStore
}

// NewRegistryCredentialHandlers creates new registry credential handlers
func NewRegistryCredentialHandlers(
	logger *slog.Logger,
	credStore *store.RegistryCredentialStore,
) *RegistryCredentialHandlers {
	return &RegistryCredentialHandlers{
		logger:    logger,
		credStore: credStore,
	}
}

// StoreCredentialRequest represents a request to store registry credentials
type StoreCredentialRequest struct {
	Registry string `json:"registry"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

// CredentialListResponse represents the response for listing credentials
type CredentialListResponse struct {
	Success     bool                       `json:"success"`
	Credentials []store.RegistryCredential `json:"credentials,omitempty"`
	Error       string                     `json:"error,omitempty"`
}

// GenericResponse represents a generic API response
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleCredentials handles POST (store), GET (list), and DELETE for /api/registry/credentials
func (h *RegistryCredentialHandlers) HandleCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.StoreCredentialHandler(w, r)
	case http.MethodGet:
		h.ListCredentialsHandler(w, r)
	case http.MethodDelete:
		h.DeleteCredentialHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// StoreCredentialHandler stores registry credentials
func (h *RegistryCredentialHandlers) StoreCredentialHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req StoreCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse store credential request", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate inputs
	if req.Registry == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Registry type is required",
		})
		return
	}

	if req.Username == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Username is required",
		})
		return
	}

	if req.Token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Token is required",
		})
		return
	}

	// Store credentials
	err := h.credStore.StoreCredential(req.Registry, req.Username, req.Token)
	if err != nil {
		h.logger.Error("Failed to store registry credentials",
			"registry", req.Registry,
			"username", req.Username,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Failed to store credentials",
		})
		return
	}

	h.logger.Info("Registry credentials stored successfully",
		"registry", req.Registry,
		"username", req.Username)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: true,
		Message: "Registry credentials stored successfully",
	})
}

// ListCredentialsHandler lists all stored credentials
func (h *RegistryCredentialHandlers) ListCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all credentials
	credentials, err := h.credStore.ListCredentials()
	if err != nil {
		h.logger.Error("Failed to list registry credentials", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CredentialListResponse{
			Success: false,
			Error:   "Failed to list credentials",
		})
		return
	}

	// Return credentials (tokens are not included for security)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(CredentialListResponse{
		Success:     true,
		Credentials: credentials,
	})
}

// DeleteCredentialHandler deletes registry credentials
func (h *RegistryCredentialHandlers) DeleteCredentialHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract registry type from URL path
	// URL format: /api/registry/credentials/{registry}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Registry type not specified in URL",
		})
		return
	}

	registryType := pathParts[3]

	if registryType == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Registry type is required",
		})
		return
	}

	// Delete credentials
	err := h.credStore.DeleteCredential(registryType)
	if err != nil {
		h.logger.Error("Failed to delete registry credentials",
			"registry", registryType,
			"error", err)

		// Check if it's a "not found" error
		statusCode := http.StatusInternalServerError
		errorMsg := "Failed to delete credentials"
		if strings.Contains(err.Error(), "no credentials found") {
			statusCode = http.StatusNotFound
			errorMsg = err.Error()
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   errorMsg,
		})
		return
	}

	h.logger.Info("Registry credentials deleted successfully",
		"registry", registryType)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: true,
		Message: "Registry credentials deleted successfully",
	})
}
