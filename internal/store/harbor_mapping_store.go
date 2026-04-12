package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Compile-time check that HarborMappingStore implements domain.HarborMappingRepository
var _ domain.HarborMappingRepository = (*HarborMappingStore)(nil)

// HarborMappingStore persists harbor mappings in SQLite.
type HarborMappingStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewHarborMappingStore creates the harbor_mappings table and returns a store.
func NewHarborMappingStore(db *sql.DB) (*HarborMappingStore, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS harbor_mappings (
			id               TEXT PRIMARY KEY,
			name             TEXT NOT NULL UNIQUE,
			project          TEXT NOT NULL,
			repository       TEXT NOT NULL,
			target_agent     TEXT NOT NULL,
			compose_template TEXT NOT NULL DEFAULT '',
			last_synced_tag  TEXT NOT NULL DEFAULT '',
			last_synced_at   INTEGER,
			created_at       INTEGER NOT NULL,
			updated_at       INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create harbor_mappings table: %w", err)
	}
	return &HarborMappingStore{db: db}, nil
}

// Get retrieves a mapping by name.
func (s *HarborMappingStore) Get(name string) (domain.HarborMapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.HarborMapping
	var lastSyncedAt *int64
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, name, project, repository, target_agent, compose_template,
		       last_synced_tag, last_synced_at, created_at, updated_at
		FROM harbor_mappings WHERE name = ?`, name).
		Scan(&m.ID, &m.Name, &m.Project, &m.Repository, &m.TargetAgent, &m.ComposeTemplate,
			&m.LastSyncedTag, &lastSyncedAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return domain.HarborMapping{}, false
	}
	if err != nil {
		return domain.HarborMapping{}, false
	}

	m.CreatedAt = time.Unix(createdAt, 0)
	m.UpdatedAt = time.Unix(updatedAt, 0)
	if lastSyncedAt != nil {
		t := time.Unix(*lastSyncedAt, 0)
		m.LastSyncedAt = &t
	}
	return m, true
}

// List returns all mappings.
func (s *HarborMappingStore) List() ([]domain.HarborMapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, project, repository, target_agent, compose_template,
		       last_synced_tag, last_synced_at, created_at, updated_at
		FROM harbor_mappings ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query harbor mappings: %w", err)
	}
	defer rows.Close()

	result := make([]domain.HarborMapping, 0)
	for rows.Next() {
		var m domain.HarborMapping
		var lastSyncedAt *int64
		var createdAt, updatedAt int64

		if err := rows.Scan(&m.ID, &m.Name, &m.Project, &m.Repository, &m.TargetAgent, &m.ComposeTemplate,
			&m.LastSyncedTag, &lastSyncedAt, &createdAt, &updatedAt); err != nil {
			continue
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		if lastSyncedAt != nil {
			t := time.Unix(*lastSyncedAt, 0)
			m.LastSyncedAt = &t
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("error iterating harbor mapping rows: %w", err)
	}
	return result, nil
}

// Create inserts a new mapping.
func (s *HarborMappingStore) Create(mapping domain.HarborMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mapping.ID == "" {
		mapping.ID = uuid.New().String()
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO harbor_mappings
			(id, name, project, repository, target_agent, compose_template, last_synced_tag, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		mapping.ID, mapping.Name, mapping.Project, mapping.Repository,
		mapping.TargetAgent, mapping.ComposeTemplate, now, now)
	if err != nil {
		return fmt.Errorf("failed to create harbor mapping: %w", err)
	}
	return nil
}

// Update saves changes to an existing mapping.
func (s *HarborMappingStore) Update(mapping domain.HarborMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	var lastSyncedAt *int64
	if mapping.LastSyncedAt != nil {
		v := mapping.LastSyncedAt.Unix()
		lastSyncedAt = &v
	}
	_, err := s.db.Exec(`
		UPDATE harbor_mappings
		SET project = ?, repository = ?, target_agent = ?, compose_template = ?,
		    last_synced_tag = ?, last_synced_at = ?, updated_at = ?
		WHERE name = ?`,
		mapping.Project, mapping.Repository, mapping.TargetAgent, mapping.ComposeTemplate,
		mapping.LastSyncedTag, lastSyncedAt, now, mapping.Name)
	if err != nil {
		return fmt.Errorf("failed to update harbor mapping: %w", err)
	}
	return nil
}

// Delete removes a mapping by name.
func (s *HarborMappingStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM harbor_mappings WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete harbor mapping: %w", err)
	}
	return nil
}
