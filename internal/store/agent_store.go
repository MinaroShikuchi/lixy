// internal/store/agent_store.go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/google/uuid"
)

// Compile-time check that AgentStore implements domain.AgentRepository
var _ domain.AgentRepository = (*AgentStore)(nil)

// AgentStore manages agent information persistence in SQLite
type AgentStore struct {
	db *sql.DB
}

func NewAgentStore(db *sql.DB) (*AgentStore, error) {
	// Create agents table if not exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
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

	return &AgentStore{
		db: db,
	}, nil
}

// Create adds a new agent to the store
func (s *AgentStore) Create(agent domain.AgentInfo) error {
	// Check if agent already exists
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM agents WHERE name = ?", agent.Name).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing agent: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("agent with name %s already exists", agent.Name)
	}

	agent.FirstSeen = time.Now()
	agent.LastSeen = time.Now()

	// Serialize maps to JSON for storage
	capabilitiesJSON, _ := json.Marshal(agent.Capabilities)
	metadataJSON, _ := json.Marshal(agent.Metadata)

	// Insert into database
	_, err = s.db.Exec(
		"INSERT INTO agents (id, name, ip, port, capabilities, first_seen, last_seen, status, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New().String(), agent.Name, agent.IP, agent.Port, string(capabilitiesJSON), agent.FirstSeen.Unix(), agent.LastSeen.Unix(), agent.Status, string(metadataJSON),
	)

	return err
}

// Upsert adds or updates agent information
func (s *AgentStore) Upsert(agent domain.AgentInfo) error {
	// Check if agent already exists to preserve first_seen
	var firstSeenUnix int64
	err := s.db.QueryRow("SELECT first_seen FROM agents WHERE name = ?", agent.Name).Scan(&firstSeenUnix)
	if err == sql.ErrNoRows {
		// New agent — set first seen time
		agent.FirstSeen = time.Now()
	} else if err != nil {
		return fmt.Errorf("failed to check existing agent: %w", err)
	} else {
		// Existing agent — preserve first seen time
		agent.FirstSeen = time.Unix(firstSeenUnix, 0)
	}

	// Always update last seen
	agent.LastSeen = time.Now()

	// Serialize maps to JSON for storage
	capabilitiesJSON, _ := json.Marshal(agent.Capabilities)
	metadataJSON, _ := json.Marshal(agent.Metadata)

	// Upsert into database
	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO agents (id, name, ip, port, capabilities, first_seen, last_seen, status, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		agent.ID, agent.Name, agent.IP, agent.Port, string(capabilitiesJSON), agent.FirstSeen.Unix(), agent.LastSeen.Unix(), agent.Status, string(metadataJSON),
	)

	return err
}

// Get retrieves agent information by name
func (s *AgentStore) Get(name string) (domain.AgentInfo, bool) {
	var agent domain.AgentInfo
	var capabilitiesJSON, metadataJSON string
	var firstSeenUnix, lastSeenUnix int64

	err := s.db.QueryRow(
		"SELECT id, name, ip, port, capabilities, first_seen, last_seen, status, metadata FROM agents WHERE name = ?",
		name,
	).Scan(&agent.ID, &agent.Name, &agent.IP, &agent.Port, &capabilitiesJSON, &firstSeenUnix, &lastSeenUnix, &agent.Status, &metadataJSON)

	if err != nil {
		return domain.AgentInfo{}, false
	}

	// Parse JSON fields
	json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities)
	json.Unmarshal([]byte(metadataJSON), &agent.Metadata)

	agent.FirstSeen = time.Unix(firstSeenUnix, 0)
	agent.LastSeen = time.Unix(lastSeenUnix, 0)

	return agent, true
}

// List returns all registered agents
func (s *AgentStore) List() []domain.AgentInfo {
	rows, err := s.db.Query("SELECT id, name, ip, port, capabilities, first_seen, last_seen, status, metadata FROM agents")
	if err != nil {
		return nil
	}
	defer rows.Close()

	agents := make([]domain.AgentInfo, 0)
	for rows.Next() {
		var agent domain.AgentInfo
		var capabilitiesJSON, metadataJSON string
		var firstSeenUnix, lastSeenUnix int64

		err := rows.Scan(&agent.ID, &agent.Name, &agent.IP, &agent.Port, &capabilitiesJSON, &firstSeenUnix, &lastSeenUnix, &agent.Status, &metadataJSON)
		if err != nil {
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities)
		json.Unmarshal([]byte(metadataJSON), &agent.Metadata)

		agent.FirstSeen = time.Unix(firstSeenUnix, 0)
		agent.LastSeen = time.Unix(lastSeenUnix, 0)

		agents = append(agents, agent)
	}

	return agents
}

// Delete removes an existing agent from the store
func (s *AgentStore) Delete(name string) error {
	result, err := s.db.Exec("DELETE FROM agents WHERE name = ?", name)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent with name %s does not exist", name)
	}

	return nil
}
