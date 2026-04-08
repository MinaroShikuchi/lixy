package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// MonitorState represents the monitoring state for a deployment
type MonitorState struct {
	ID             string     `json:"id"`
	DeploymentName string     `json:"deployment_name"`
	Environment    string     `json:"environment"`
	LastCheck      time.Time  `json:"last_check"`
	LastUpdate     *time.Time `json:"last_update,omitempty"`
	CheckCount     int        `json:"check_count"`
	UpdateCount    int        `json:"update_count"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// MonitorStateStore manages monitoring state for deployments
type MonitorStateStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewMonitorStateStore creates a new monitor state store
func NewMonitorStateStore(db *sql.DB) (*MonitorStateStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Create monitor_state table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS monitor_state (
			id TEXT PRIMARY KEY,
			deployment_name TEXT NOT NULL UNIQUE,
			environment TEXT NOT NULL,
			last_check INTEGER NOT NULL,
			last_update INTEGER,
			check_count INTEGER NOT NULL DEFAULT 0,
			update_count INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor_state table: %w", err)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_monitor_state_enabled 
		 ON monitor_state(enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_state_last_check 
		 ON monitor_state(last_check)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_state_deployment 
		 ON monitor_state(deployment_name, environment)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return nil, fmt.Errorf("failed to create index: %w", err)
		}
	}

	return &MonitorStateStore{db: db}, nil
}

// UpdateCheckTime updates the last check time for a deployment
func (s *MonitorStateStore) UpdateCheckTime(deploymentName, environment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Try to update existing record
	result, err := s.db.Exec(`
		UPDATE monitor_state
		SET last_check = ?, check_count = check_count + 1, updated_at = ?
		WHERE deployment_name = ? AND environment = ?
	`, now.Unix(), now.Unix(), deploymentName, environment)

	if err != nil {
		return fmt.Errorf("failed to update check time: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows updated, create new record
	if rowsAffected == 0 {
		id := uuid.New().String()
		_, err = s.db.Exec(`
			INSERT INTO monitor_state (
				id, deployment_name, environment, last_check, last_update,
				check_count, update_count, enabled, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, deploymentName, environment, now.Unix(), nil, 1, 0, true, now.Unix(), now.Unix())

		if err != nil {
			return fmt.Errorf("failed to create monitor state: %w", err)
		}
	}

	return nil
}

// UpdateUpdateTime updates the last update time for a deployment
func (s *MonitorStateStore) UpdateUpdateTime(deploymentName, environment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	result, err := s.db.Exec(`
		UPDATE monitor_state
		SET last_update = ?, update_count = update_count + 1, updated_at = ?
		WHERE deployment_name = ? AND environment = ?
	`, now.Unix(), now.Unix(), deploymentName, environment)

	if err != nil {
		return fmt.Errorf("failed to update update time: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("monitor state not found for deployment: %s/%s", environment, deploymentName)
	}

	return nil
}

// GetState retrieves the monitor state for a deployment
func (s *MonitorStateStore) GetState(deploymentName, environment string) (*MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, deployment_name, environment, last_check, last_update,
		       check_count, update_count, enabled, created_at, updated_at
		FROM monitor_state
		WHERE deployment_name = ? AND environment = ?
	`

	row := s.db.QueryRow(query, deploymentName, environment)

	var state MonitorState
	var lastCheckUnix, createdAtUnix, updatedAtUnix int64
	var lastUpdateUnix sql.NullInt64

	err := row.Scan(
		&state.ID,
		&state.DeploymentName,
		&state.Environment,
		&lastCheckUnix,
		&lastUpdateUnix,
		&state.CheckCount,
		&state.UpdateCount,
		&state.Enabled,
		&createdAtUnix,
		&updatedAtUnix,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found, but not an error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor state: %w", err)
	}

	state.LastCheck = time.Unix(lastCheckUnix, 0)
	state.CreatedAt = time.Unix(createdAtUnix, 0)
	state.UpdatedAt = time.Unix(updatedAtUnix, 0)

	if lastUpdateUnix.Valid {
		lastUpdate := time.Unix(lastUpdateUnix.Int64, 0)
		state.LastUpdate = &lastUpdate
	}

	return &state, nil
}

// EnableMonitoring enables or disables monitoring for a deployment
func (s *MonitorStateStore) EnableMonitoring(deploymentName, environment string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Try to update existing record
	result, err := s.db.Exec(`
		UPDATE monitor_state
		SET enabled = ?, updated_at = ?
		WHERE deployment_name = ? AND environment = ?
	`, enabled, now.Unix(), deploymentName, environment)

	if err != nil {
		return fmt.Errorf("failed to update monitoring state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows updated, create new record
	if rowsAffected == 0 {
		id := uuid.New().String()
		_, err = s.db.Exec(`
			INSERT INTO monitor_state (
				id, deployment_name, environment, last_check, last_update,
				check_count, update_count, enabled, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, deploymentName, environment, now.Unix(), nil, 0, 0, enabled, now.Unix(), now.Unix())

		if err != nil {
			return fmt.Errorf("failed to create monitor state: %w", err)
		}
	}

	return nil
}

// GetActiveDeployments retrieves all deployments with monitoring enabled
func (s *MonitorStateStore) GetActiveDeployments() ([]MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, deployment_name, environment, last_check, last_update,
		       check_count, update_count, enabled, created_at, updated_at
		FROM monitor_state
		WHERE enabled = 1
		ORDER BY deployment_name, environment
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active deployments: %w", err)
	}
	defer rows.Close()

	return s.scanStates(rows)
}

// GetAllStates retrieves all monitor states
func (s *MonitorStateStore) GetAllStates() ([]MonitorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, deployment_name, environment, last_check, last_update,
		       check_count, update_count, enabled, created_at, updated_at
		FROM monitor_state
		ORDER BY deployment_name, environment
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all states: %w", err)
	}
	defer rows.Close()

	return s.scanStates(rows)
}

// scanStates is a helper to scan multiple state records
func (s *MonitorStateStore) scanStates(rows *sql.Rows) ([]MonitorState, error) {
	var states []MonitorState

	for rows.Next() {
		var state MonitorState
		var lastCheckUnix, createdAtUnix, updatedAtUnix int64
		var lastUpdateUnix sql.NullInt64

		err := rows.Scan(
			&state.ID,
			&state.DeploymentName,
			&state.Environment,
			&lastCheckUnix,
			&lastUpdateUnix,
			&state.CheckCount,
			&state.UpdateCount,
			&state.Enabled,
			&createdAtUnix,
			&updatedAtUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan state record: %w", err)
		}

		state.LastCheck = time.Unix(lastCheckUnix, 0)
		state.CreatedAt = time.Unix(createdAtUnix, 0)
		state.UpdatedAt = time.Unix(updatedAtUnix, 0)

		if lastUpdateUnix.Valid {
			lastUpdate := time.Unix(lastUpdateUnix.Int64, 0)
			state.LastUpdate = &lastUpdate
		}

		states = append(states, state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating states: %w", err)
	}

	return states, nil
}

// DeleteState deletes the monitor state for a deployment
func (s *MonitorStateStore) DeleteState(deploymentName, environment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		DELETE FROM monitor_state
		WHERE deployment_name = ? AND environment = ?
	`, deploymentName, environment)

	if err != nil {
		return fmt.Errorf("failed to delete monitor state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("monitor state not found for deployment: %s/%s", environment, deploymentName)
	}

	return nil
}

// Close closes the database connection
func (s *MonitorStateStore) Close() error {
	return s.db.Close()
}
