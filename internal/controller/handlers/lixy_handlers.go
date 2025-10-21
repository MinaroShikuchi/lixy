package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/auth"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

var agentStore *store.AgentStore

func SetAgentStore(store *store.AgentStore) {
	agentStore = store
}

// GenerateRegistrationTokenHandler creates a time-limited token for agent registration
func GenerateRegistrationTokenHandler(w http.ResponseWriter, r *http.Request) {
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
	token, err := auth.GenerateRegistrationToken(duration)
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
func RegisterAgentHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received %s request for %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token     string            `json:"token"`
		AgentName string            `json:"agent_name"`
		AgentInfo map[string]string `json:"agent_info"`
		Version   string            `json:"version"`
		IP        string            `json:"ip"`
		Port      int               `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate registration token
	if _, err := auth.ValidateRegistrationToken(req.Token); err != nil {
		http.Error(w, "Invalid or expired registration token", http.StatusUnauthorized)
		return
	}

	// Generate a new agent ID
	agentID := generateAgentID()

	// Generate permanent token for agent
	permanentToken, err := auth.GeneratePermanentToken(agentID, req.AgentName)
	if err != nil {
		http.Error(w, "Failed to generate permanent token", http.StatusInternalServerError)
		return
	}

	// Store agent information
	agent := store.AgentInfo{
		ID:       agentID,
		Name:     req.AgentName,
		IP:       req.IP,
		Port:     req.Port,
		Status:   "online",
		Metadata: map[string]string{}, // Can be populated from more claims if needed
	}

	if err := agentStore.UpsertAgent(agent); err != nil {
		http.Error(w, "Failed to store agent information", http.StatusInternalServerError)
		return
	}

	// Return success with permanent token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"agent_id": agentID,
		"token":    permanentToken,
	})
}

// Helper function to generate a unique agent ID
func generateAgentID() string {
	return fmt.Sprintf("agent-%d", time.Now().UnixNano())
}

// Handler to list all registered agents
func ListAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all agents from the store
	agents := agentStore.ListAgents()

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}
