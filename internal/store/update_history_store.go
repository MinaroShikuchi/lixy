package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// UpdateHistory represents a record of an image update check or application
type UpdateHistory struct {
	ID             string     `json:"id"`
	DeploymentName string     `json:"deployment_name"`
	Environment    string     `json:"environment"`
	Image          string     `json:"image"`
	OldTag         string     `json:"old_tag"`
	NewTag         string     `json:"new_tag"`
	Strategy       string     `json:"strategy"`
	Status         string     `json:"status"` // 'pending', 'applied', 'failed', 'rolled_back'
	AutoApplied    bool       `json:"auto_applied"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CheckedAt      time.Time  `json:"checked_at"`
	AppliedAt      *time.Time `json:"applied_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// UpdateHistoryStore manages update history records
type UpdateHistoryStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewUpdateHistoryStore creates a new update history store
func NewUpdateHistoryStore(db *sql.DB) (*UpdateHistoryStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Create update_history table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS update_history (
			id TEXT PRIMARY KEY,
			deployment_name TEXT NOT NULL,
			environment TEXT NOT NULL,
			image TEXT NOT NULL,
			old_tag TEXT NOT NULL,
			new_tag TEXT NOT NULL,
			strategy TEXT NOT NULL,
			status TEXT NOT NULL,
			auto_applied BOOLEAN NOT NULL,
			error_message TEXT,
			checked_at INTEGER NOT NULL,
			applied_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create update_history table: %w", err)
	}

	// Create indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_update_history_deployment 
		 ON update_history(deployment_name, environment)`,
		`CREATE INDEX IF NOT EXISTS idx_update_history_status 
		 ON update_history(status)`,
		`CREATE INDEX IF NOT EXISTS idx_update_history_checked_at 
		 ON update_history(checked_at DESC)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return nil, fmt.Errorf("failed to create index: %w", err)
		}
	}

	return &UpdateHistoryStore{db: db}, nil
}

// RecordCheck records an update check (whether update was found or not)
func (s *UpdateHistoryStore) RecordCheck(deploymentName, environment, image, oldTag, newTag, strategy string, updateFound bool) (*UpdateHistory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	record := &UpdateHistory{
		ID:             uuid.New().String(),
		DeploymentName: deploymentName,
		Environment:    environment,
		Image:          image,
		OldTag:         oldTag,
		NewTag:         newTag,
		Strategy:       strategy,
		Status:         "pending",
		AutoApplied:    false,
		CheckedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// If no update found, mark as no-op
	if !updateFound {
		record.Status = "no_update"
	}

	_, err := s.db.Exec(`
		INSERT INTO update_history (
			id, deployment_name, environment, image, old_tag, new_tag,
			strategy, status, auto_applied, error_message, checked_at,
			applied_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ID,
		record.DeploymentName,
		record.Environment,
		record.Image,
		record.OldTag,
		record.NewTag,
		record.Strategy,
		record.Status,
		record.AutoApplied,
		record.ErrorMessage,
		record.CheckedAt.Unix(),
		nil, // applied_at is null initially
		record.CreatedAt.Unix(),
		record.UpdatedAt.Unix(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to record check: %w", err)
	}

	return record, nil
}

// RecordUpdate records a successful update application
func (s *UpdateHistoryStore) RecordUpdate(id string, autoApplied bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	result, err := s.db.Exec(`
		UPDATE update_history
		SET status = ?, auto_applied = ?, applied_at = ?, updated_at = ?
		WHERE id = ?
	`, "applied", autoApplied, now.Unix(), now.Unix(), id)

	if err != nil {
		return fmt.Errorf("failed to record update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("update record not found: %s", id)
	}

	return nil
}

// UpdateStatus updates the status of an update record
func (s *UpdateHistoryStore) UpdateStatus(id, status, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	result, err := s.db.Exec(`
		UPDATE update_history
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, status, errorMessage, now.Unix(), id)

	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("update record not found: %s", id)
	}

	return nil
}

// GetHistory retrieves update history for a deployment
func (s *UpdateHistoryStore) GetHistory(deploymentName, environment string, limit int) ([]UpdateHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, deployment_name, environment, image, old_tag, new_tag,
		       strategy, status, auto_applied, error_message, checked_at,
		       applied_at, created_at, updated_at
		FROM update_history
		WHERE deployment_name = ? AND environment = ?
		ORDER BY checked_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, deploymentName, environment, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	return s.scanHistory(rows)
}

// GetRecentUpdates retrieves recent updates across all deployments
func (s *UpdateHistoryStore) GetRecentUpdates(limit int) ([]UpdateHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, deployment_name, environment, image, old_tag, new_tag,
		       strategy, status, auto_applied, error_message, checked_at,
		       applied_at, created_at, updated_at
		FROM update_history
		ORDER BY checked_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent updates: %w", err)
	}
	defer rows.Close()

	return s.scanHistory(rows)
}

// GetPendingUpdates retrieves all pending updates
func (s *UpdateHistoryStore) GetPendingUpdates() ([]UpdateHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, deployment_name, environment, image, old_tag, new_tag,
		       strategy, status, auto_applied, error_message, checked_at,
		       applied_at, created_at, updated_at
		FROM update_history
		WHERE status = 'pending'
		ORDER BY checked_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending updates: %w", err)
	}
	defer rows.Close()

	return s.scanHistory(rows)
}

// GetByID retrieves a specific update record by ID
func (s *UpdateHistoryStore) GetByID(id string) (*UpdateHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, deployment_name, environment, image, old_tag, new_tag,
		       strategy, status, auto_applied, error_message, checked_at,
		       applied_at, created_at, updated_at
		FROM update_history
		WHERE id = ?
	`

	row := s.db.QueryRow(query, id)

	var record UpdateHistory
	var checkedAtUnix, createdAtUnix, updatedAtUnix int64
	var appliedAtUnix sql.NullInt64
	var errorMessage sql.NullString

	err := row.Scan(
		&record.ID,
		&record.DeploymentName,
		&record.Environment,
		&record.Image,
		&record.OldTag,
		&record.NewTag,
		&record.Strategy,
		&record.Status,
		&record.AutoApplied,
		&errorMessage,
		&checkedAtUnix,
		&appliedAtUnix,
		&createdAtUnix,
		&updatedAtUnix,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("update record not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get update record: %w", err)
	}

	record.CheckedAt = time.Unix(checkedAtUnix, 0)
	record.CreatedAt = time.Unix(createdAtUnix, 0)
	record.UpdatedAt = time.Unix(updatedAtUnix, 0)

	if appliedAtUnix.Valid {
		appliedAt := time.Unix(appliedAtUnix.Int64, 0)
		record.AppliedAt = &appliedAt
	}

	if errorMessage.Valid {
		record.ErrorMessage = errorMessage.String
	}

	return &record, nil
}

// scanHistory is a helper to scan multiple history records
func (s *UpdateHistoryStore) scanHistory(rows *sql.Rows) ([]UpdateHistory, error) {
	var history []UpdateHistory

	for rows.Next() {
		var record UpdateHistory
		var checkedAtUnix, createdAtUnix, updatedAtUnix int64
		var appliedAtUnix sql.NullInt64
		var errorMessage sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.DeploymentName,
			&record.Environment,
			&record.Image,
			&record.OldTag,
			&record.NewTag,
			&record.Strategy,
			&record.Status,
			&record.AutoApplied,
			&errorMessage,
			&checkedAtUnix,
			&appliedAtUnix,
			&createdAtUnix,
			&updatedAtUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history record: %w", err)
		}

		record.CheckedAt = time.Unix(checkedAtUnix, 0)
		record.CreatedAt = time.Unix(createdAtUnix, 0)
		record.UpdatedAt = time.Unix(updatedAtUnix, 0)

		if appliedAtUnix.Valid {
			appliedAt := time.Unix(appliedAtUnix.Int64, 0)
			record.AppliedAt = &appliedAt
		}

		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}

		history = append(history, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating history: %w", err)
	}

	return history, nil
}

// DeleteOldRecords deletes records older than the specified duration
func (s *UpdateHistoryStore) DeleteOldRecords(olderThan time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan).Unix()

	result, err := s.db.Exec(`
		DELETE FROM update_history
		WHERE checked_at < ?
	`, cutoff)

	if err != nil {
		return 0, fmt.Errorf("failed to delete old records: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// Close closes the database connection
func (s *UpdateHistoryStore) Close() error {
	return s.db.Close()
}
