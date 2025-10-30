package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

func (ah *AgentHandlers) SaveAgentHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.SaveAgentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Error decoding save agent request", slog.String("error", err.Error()))
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	agentName := r.Context().Value("agent_name").(string)
	if err := ah.agentService.CreateAgent(agentName, req.IP, req.Port); err != nil {
		logger.Error("Failed to save agent information", slog.String("error", err.Error()))
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

// Helper function to generate a unique agent name
func generateAgentName(hostname string) string {
	return fmt.Sprintf("agent-%s", hostname)
}

// Handler to list all registered agents
func (ah *AgentHandlers) ListAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all agents from the store
	agents := ah.agentService.ListAgents()

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}
