package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Compile-time check that DeploymentStore implements domain.DeploymentRepository
var _ domain.DeploymentRepository = (*DeploymentStore)(nil)

// DeploymentStore manages deployments persisted in SQLite
type DeploymentStore struct {
	deployments []domain.DeploymentInfo
	mu          sync.RWMutex
	db          *sql.DB
}

// NewDeploymentStore creates a new DeploymentStore backed by SQLite
func NewDeploymentStore(db *sql.DB) (*DeploymentStore, error) {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS deployments (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            target_lxc TEXT NOT NULL,
            compose_yml TEXT NOT NULL,
            status TEXT NOT NULL,
            env_vars TEXT NOT NULL DEFAULT '{}',
            FOREIGN KEY (target_lxc) REFERENCES agents(name) ON DELETE RESTRICT ON UPDATE CASCADE
        )
    `)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create deployments table: %w", err)
	}

	// Migrate existing databases that don't have the env_vars column yet
	_, _ = db.Exec(`ALTER TABLE deployments ADD COLUMN env_vars TEXT NOT NULL DEFAULT '{}'`)

	return &DeploymentStore{
		deployments: []domain.DeploymentInfo{},
		mu:          sync.RWMutex{},
		db:          db,
	}, nil
}

// marshalEnvVars serialises an env vars map to a JSON string for storage.
func marshalEnvVars(envVars map[string]string) string {
	if len(envVars) == 0 {
		return "{}"
	}
	b, err := json.Marshal(envVars)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// unmarshalEnvVars deserialises a JSON string back into an env vars map.
func unmarshalEnvVars(s string) map[string]string {
	result := make(map[string]string)
	if s == "" || s == "{}" {
		return result
	}
	_ = json.Unmarshal([]byte(s), &result)
	return result
}

// ListDeployments returns all deployments from the DB
func (ds *DeploymentStore) List() ([]domain.DeploymentInfo, error) {
	if ds == nil || ds.db == nil {
		return nil, fmt.Errorf("deployment store is not initialized")
	}
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	rows, err := ds.db.Query(`SELECT id, name, target_lxc, status, compose_yml, env_vars FROM deployments`)
	if err != nil {
		return nil, fmt.Errorf("failed to query deployments: %w", err)
	}
	defer rows.Close()

	result := make([]domain.DeploymentInfo, 0)
	for rows.Next() {
		var d domain.DeploymentInfo
		var envVarsJSON string
		if err := rows.Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status, &d.ComposeYAML, &envVarsJSON); err != nil {
			continue
		}
		d.EnvVars = unmarshalEnvVars(envVarsJSON)
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("error iterating deployment rows: %w", err)
	}
	return result, nil
}

// GetDeployment retrieves a deployment by name
func (ds *DeploymentStore) Get(name string) (domain.DeploymentInfo, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var d domain.DeploymentInfo
	var envVarsJSON string
	err := ds.db.QueryRow(`SELECT id, name, target_lxc, status, compose_yml, env_vars FROM deployments WHERE name = ?`, name).
		Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status, &d.ComposeYAML, &envVarsJSON)
	if err == sql.ErrNoRows {
		return domain.DeploymentInfo{}, false
	}
	if err != nil {
		return domain.DeploymentInfo{}, false
	}
	d.EnvVars = unmarshalEnvVars(envVarsJSON)
	return d, true
}

// Create inserts a new deployment into the DB. Returns an error if a deployment with the same name already exists.
func (ds *DeploymentStore) Create(deployment domain.DeploymentInfo) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	id := uuid.New().String()
	_, err := ds.db.Exec(
		`INSERT INTO deployments (id, name, target_lxc, compose_yml, status, env_vars) VALUES (?, ?, ?, ?, ?, ?)`,
		id, deployment.Name, deployment.TargetLXC, deployment.ComposeYAML, deployment.Status, marshalEnvVars(deployment.EnvVars),
	)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	return nil
}

// CreateOrUpdate adds or updates a deployment in the DB
func (ds *DeploymentStore) CreateOrUpdate(deployment domain.DeploymentInfo) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	envVarsJSON := marshalEnvVars(deployment.EnvVars)

	res, err := ds.db.Exec(
		`UPDATE deployments SET target_lxc = ?, compose_yml = ?, status = ?, env_vars = ? WHERE name = ?`,
		deployment.TargetLXC, deployment.ComposeYAML, deployment.Status, envVarsJSON, deployment.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to add or update deployment: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to add or update deployment: %w", err)
	}
	if ra > 0 {
		return nil
	}

	id := uuid.New().String()
	_, err = ds.db.Exec(
		`INSERT INTO deployments (id, name, target_lxc, compose_yml, status, env_vars) VALUES (?, ?, ?, ?, ?, ?)`,
		id, deployment.Name, deployment.TargetLXC, deployment.ComposeYAML, deployment.Status, envVarsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to add deployment: %w", err)
	}
	return nil
}

// Update Deployment updates an existing deployment in the DB
func (ds *DeploymentStore) Update(deployment domain.DeploymentInfo) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, err := ds.db.Exec(
		`UPDATE deployments SET compose_yml = ?, status = ?, env_vars = ? WHERE name = ?`,
		deployment.ComposeYAML, deployment.Status, marshalEnvVars(deployment.EnvVars), deployment.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}
	return nil
}

// Removes a deployment by name from the DB
func (ds *DeploymentStore) Delete(name string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, err := ds.db.Exec(`DELETE FROM deployments WHERE name = ?`, name)
	if err != nil {
		return err
	}
	return nil
}
