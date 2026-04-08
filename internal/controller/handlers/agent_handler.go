package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type AgentHandlers struct {
	agentService *services.AgentService
	authService  *services.AuthService
	logger       *slog.Logger
}

func NewAgentHandlers(agentService *services.AgentService, authService *services.AuthService, logger *slog.Logger) *AgentHandlers {
	return &AgentHandlers{
		agentService: agentService,
		authService:  authService,
		logger:       logger,
	}
}

func (ah *AgentHandlers) SaveAgentHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.SaveAgentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ah.logger.Error("Error decoding save agent request", slog.String("error", err.Error()))
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	agentName, ok := r.Context().Value(domain.AgentNameKey).(string)
	if !ok {
		ah.logger.Error("Agent name not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := ah.agentService.CreateAgent(agentName, req.IP, req.Port); err != nil {
		ah.logger.Error("Failed to save agent information", slog.String("error", err.Error()))
		http.Error(w, "Failed to save agent information", http.StatusInternalServerError)
		return
	}

	// Return success with permanent token
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/agents/%s", agentName))
	w.WriteHeader(http.StatusCreated)

	resp := domain.SaveAgentResponse{
		Success: true,
		Message: "Agent registered successfully",
		Data:    map[string]string{"agent_name": agentName},
	}

	json.NewEncoder(w).Encode(resp)
}

// Handler to list all registered agents
func (ah *AgentHandlers) ListAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all agents from the store
	agents := ah.agentService.ListAgents()

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// UnrgisterAgentHandler handles agent registration
func (ah *AgentHandlers) UnregisterAgentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentName := r.Context().Value(domain.AgentNameKey).(string)

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
