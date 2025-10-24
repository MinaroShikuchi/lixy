package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

func (ah *AgentHandlers) SaveAgentHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request for %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req domain.SaveAgentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding registration request: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	agentID := r.Context().Value("agent_id").(string)

	if err := ah.agentService.CreateAgent(agentID, req.AgentName, req.IP, req.Port); err != nil {
		log.Printf("Error saving agent info: %v", err)
		http.Error(w, "Failed to save agent information", http.StatusInternalServerError)
		return
	}

	// Return success with permanent token
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/agents/%s", agentID))
	w.WriteHeader(http.StatusCreated)

	resp := domain.Response{
		Success: true,
		Message: "Agent registered successfully",
	}

	json.NewEncoder(w).Encode(resp)
}

// Helper function to generate a unique agent ID
func generateAgentID(hostname string) string {
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
