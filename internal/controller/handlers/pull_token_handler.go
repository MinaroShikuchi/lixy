package handlers

import (
	"encoding/json"
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
		json.NewEncoder(w).Encode(domain.PullTokenAPIResponse{
			Success: false,
			Error:   "Agent not authenticated",
		})
		return
	}

	// Parse request
	var req domain.PullTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse pull token request",
			"agent", agentName,
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain.PullTokenAPIResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate registry
	if req.Registry == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(domain.PullTokenAPIResponse{
			Success: false,
			Error:   "Registry is required",
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
		json.NewEncoder(w).Encode(domain.PullTokenAPIResponse{
			Success: false,
			Error:   "Failed to generate pull token",
		})
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(domain.PullTokenAPIResponse{
		Success:   true,
		Token:     tokenResp.Token,
		Username:  tokenResp.Username,
		Registry:  tokenResp.Registry,
		ExpiresIn: tokenResp.ExpiresIn,
		ExpiresAt: tokenResp.ExpiresAt.Format(time.RFC3339),
	})
}
