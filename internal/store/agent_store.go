// internal/store/agent_store.go
package store

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentInfo represents the registered agent information
type AgentInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	IP           string            `json:"ip"`
	Port         int               `json:"port"`
	Capabilities map[string]string `json:"capabilities"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
	Status       string            `json:"status"` // "online", "offline", "unreachable"
	Metadata     map[string]string `json:"metadata"`
}

// AgentStore manages agent information persistence
type AgentStore struct {
	agents map[string]AgentInfo
	mutex  sync.RWMutex
	db     *sql.DB // If using a database
}

func NewAgentStore() (*AgentStore, error) {
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize SQLite token store
	dbPath := filepath.Join(dataDir, "agents.db")
	// Initialize database connection if using a database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create agents table if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL,
			capabilities TEXT NOT NULL,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			status TEXT NOT NULL,
			metadata TEXT NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	// Load existing agents into memory
	agents := make(map[string]AgentInfo)
	rows, err := db.Query("SELECT id, name, ip, port, capabilities, first_seen, last_seen, status, metadata FROM agents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var agent AgentInfo
		var capabilitiesJSON, metadataJSON string
		var firstSeenUnix, lastSeenUnix int64

		err := rows.Scan(&agent.ID, &agent.Name, &agent.IP, &agent.Port, &capabilitiesJSON, &firstSeenUnix, &lastSeenUnix, &agent.Status, &metadataJSON)
		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities)
		json.Unmarshal([]byte(metadataJSON), &agent.Metadata)

		agent.FirstSeen = time.Unix(firstSeenUnix, 0)
		agent.LastSeen = time.Unix(lastSeenUnix, 0)

		agents[agent.ID] = agent
	}

	return &AgentStore{
		agents: agents,
		mutex:  sync.RWMutex{},
		db:     db,
	}, nil
}

// UpsertAgent adds or updates agent information
func (s *AgentStore) UpsertAgent(agent AgentInfo) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if agent already exists
	existing, exists := s.agents[agent.ID]
	if exists {
		// Preserve first seen time if agent already exists
		agent.FirstSeen = existing.FirstSeen
	} else {
		// Set first seen time for new agents
		agent.FirstSeen = time.Now()
	}

	// Always update last seen
	agent.LastSeen = time.Now()

	// Update in-memory cache
	s.agents[agent.ID] = agent

	// Serialize maps to JSON for storage
	capabilitiesJSON, _ := json.Marshal(agent.Capabilities)
	metadataJSON, _ := json.Marshal(agent.Metadata)

	// Update database
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO agents (id, name, ip, port, capabilities, first_seen, last_seen, status, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		agent.ID, agent.Name, agent.IP, agent.Port, string(capabilitiesJSON), agent.FirstSeen.Unix(), agent.LastSeen.Unix(), agent.Status, string(metadataJSON),
	)

	return err
}

// GetAgent retrieves agent information by ID
func (s *AgentStore) GetAgent(id string) (AgentInfo, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	agent, exists := s.agents[id]
	return agent, exists
}

// ListAgents returns all registered agents
func (s *AgentStore) ListAgents() []AgentInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	agents := make([]AgentInfo, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}

	return agents
}
