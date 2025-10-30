package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type AgentHandlers struct {
	agentService *services.AgentService
}

func NewAgentHandlers(agentService *services.AgentService) *AgentHandlers {
	return &AgentHandlers{
		agentService: agentService,
	}
}

// GenerateRegistrationTokenHandler creates a time-limited token for agent registration
func (ah *AgentHandlers) GenerateRegistrationTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req struct {
		Expiration string `json:"expiration"` // e.g., "1h", "30m"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Parse expiration
	duration, err := time.ParseDuration(req.Expiration)
	if err != nil {
		http.Error(w, "Invalid expiration format", http.StatusBadRequest)
		return
	}

	// Generate token
	token, err := services.GenerateRegistrationToken(duration)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_in": int(duration.Seconds()),
	})
}

// RegisterAgentHandler handles agent registration with a token
func (ah *AgentHandlers) RegisterAgentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.RegistrationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Error decoding registration request", slog.String("error", err.Error()))
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate registration token
	if _, err := services.ValidateRegistrationToken(req.Token); err != nil {
		logger.Error("Agent registration failed", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Generate a new agent name
	agentName := generateAgentName(req.Hostname)
	// Generate permanent token for agent
	permanentToken, err := services.GeneratePermanentToken(agentName)
	if err != nil {
		http.Error(w, "Failed to generate permanent token", http.StatusInternalServerError)
		return
	}
	logger.Debug("Agent registered successfully", slog.String("agent_name", agentName))
	// Return success with permanent token
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(domain.RegistrationResponse{
		Success: true,
		Message: "Agent registered successfully",
		Token:   permanentToken,
	})
}

// UnrgisterAgentHandler handles agent registration
func (ah *AgentHandlers) UnregisterAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentName := r.Context().Value("agent_name").(string)

	if err := ah.agentService.DeleteAgent(agentName); err != nil {
		http.Error(w, "Failed to unregister agent", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": "true",
		"message": "Agent unregistered successfully",
	})
}
