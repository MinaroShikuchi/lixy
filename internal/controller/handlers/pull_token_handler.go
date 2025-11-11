package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

// PullTokenHandlers handles pull token requests
type PullTokenHandlers struct {
	logger           *slog.Logger
	pullTokenService *services.PullTokenService
}

// NewPullTokenHandlers creates new pull token handlers
func NewPullTokenHandlers(
	logger *slog.Logger,
	pullTokenService *services.PullTokenService,
) *PullTokenHandlers {
	return &PullTokenHandlers{
		logger:           logger,
		pullTokenService: pullTokenService,
	}
}

// PullTokenRequest represents a pull token request
type PullTokenRequest struct {
	Registry string `json:"registry"`
}

// PullTokenAPIResponse represents the API response for pull token requests
type PullTokenAPIResponse struct {
	Success   bool   `json:"success"`
	Token     string `json:"token,omitempty"`
	Username  string `json:"username,omitempty"`
	Registry  string `json:"registry,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RequestPullTokenHandler handles pull token requests from agents
func (h *PullTokenHandlers) RequestPullTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract agent name from context (set by auth middleware)
	agentName, ok := r.Context().Value(domain.AgentNameKey).(string)
	// debug log if agent name is missing
	h.logger.Debug("Extracted agent name from context", "agentName", agentName, "ok", ok)

	if !ok || agentName == "" {
		h.logger.Warn("Pull token request without agent authentication")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(PullTokenAPIResponse{
			Success: false,
			Error:   "Agent not authenticated",
		})
		return
	}

	// Parse request
	var req PullTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse pull token request",
			"agent", agentName,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PullTokenAPIResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate and check registry type
	switch req.Registry {
	case "":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PullTokenAPIResponse{
			Success: false,
			Error:   "Registry type is required",
		})
		return
	case "ghcr.io":
		// Supported registry, continue
	default:
		h.logger.Warn("Unsupported registry type requested",
			"agent", agentName,
			"registry", req.Registry)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(PullTokenAPIResponse{
			Success: false,
			Error:   fmt.Sprintf("Unsupported registry type '%s'. Currently only 'ghcr.io' is supported", req.Registry),
		})
		return
	}

	// Generate pull token
	tokenResp, err := h.pullTokenService.GeneratePullToken(agentName, req.Registry)
	if err != nil {
		h.logger.Error("Failed to generate pull token",
			"agent", agentName,
			"registry", req.Registry,
			"error", err)

		// Check if it's a "credentials not found" error

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(PullTokenAPIResponse{
			Success: false,
			Error:   "Failed to generate pull token",
		})
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(PullTokenAPIResponse{
		Success:   true,
		Token:     tokenResp.Token,
		Username:  tokenResp.Username,
		Registry:  tokenResp.Registry,
		ExpiresIn: tokenResp.ExpiresIn,
		ExpiresAt: tokenResp.ExpiresAt.Format(time.RFC3339),
	})
}
