// internal/controller/health.go
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
)

// HealthCheckResult represents the response from a lixies health check endpoint
type HealthCheckResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// CheckAgentHealth verifies if a lixies agent is alive by calling its healthz endpoint
func CheckAgentHealth(agent store.AgentInfo, agentStore *store.AgentStore) (bool, string) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Build the health check URL
	// fmt.Printf("Checking health of agent %s at %s:%d\n", agent.ID, agent.IP, agent.Port)
	healthURL := fmt.Sprintf("http://%s:%d/healthz", agent.IP, agent.Port)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		// Update agent status via agentStore
		agent.Status = "unreachable"
		agent.Metadata["health_message"] = fmt.Sprintf("Failed to create request: %v", err)
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return false, fmt.Sprintf("Failed to create request: %v", err)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		// Update agent status via agentStore
		agent.Status = "offline"
		agent.Metadata["health_message"] = fmt.Sprintf("Failed to connect: %v", err)
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return false, fmt.Sprintf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		// Update agent status via agentStore
		agent.Status = "degraded"
		agent.Metadata["health_message"] = fmt.Sprintf("Unhealthy status code: %d", resp.StatusCode)
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return false, fmt.Sprintf("Unhealthy status code: %d", resp.StatusCode)
	}

	// Parse health check response
	var result HealthCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		agent.Status = "degraded"
		agent.Metadata["health_message"] = fmt.Sprintf("Invalid response format: %v", err)
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return false, fmt.Sprintf("Invalid response format: %v", err)
	}

	// Update agent status based on response
	if result.Healthy {
		agent.Status = "online"
		agent.Metadata["health_message"] = result.Message
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return true, result.Message
	} else {
		agent.Status = "degraded"
		agent.Metadata["health_message"] = result.Message
		agent.LastSeen = time.Now()
		agentStore.Upsert(agent)

		return false, result.Message
	}
}

// updateAgentStatus updates the agent's status in the database
func updateAgentStatus(agentID, status, message string, db *sql.DB) error {
	// Update the agent's status and last_seen timestamp
	_, err := db.Exec(
		"UPDATE agents SET status = ?, message = ?, last_seen = ? WHERE id = ?",
		status, message, time.Now().Unix(), agentID,
	)

	return err
}
