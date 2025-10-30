package store

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// DeploymentInfo represents information about a deployment with docker compose
type DeploymentInfo struct {
	ID          string
	Name        string
	TargetLXC   string
	ComposeYAML []byte
	Status      string
}

// DeploymentStore manages deployments persisted in SQLite
type DeploymentStore struct {
	deployments []DeploymentInfo
	mu          sync.RWMutex
	db          *sql.DB
}

// NewDeploymentStore creates a new DeploymentStore backed by SQLite
func NewDeploymentStore(db *sql.DB) (*DeploymentStore, error) {
	// Create deployments table with foreign key constraint on target_lxc
	// Note: this references the lxc_targets table — ensure that table exists with a "name" column.
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS deployments (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            target_lxc TEXT NOT NULL,
            compose_yml TEXT NOT NULL,
            status TEXT NOT NULL,
            FOREIGN KEY (target_lxc) REFERENCES agents(name) ON DELETE RESTRICT ON UPDATE CASCADE
        )
    `)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create deployments table: %w", err)
	}

	// Load existing deployments into memory
	deployments := []DeploymentInfo{}
	rows, err := db.Query(`SELECT id, name, target_lxc, status FROM deployments`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to query existing deployments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deployment DeploymentInfo
		err := rows.Scan(&deployment.ID, &deployment.Name, &deployment.TargetLXC, &deployment.Status)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to scan deployment row: %w", err)
		}
		deployments = append(deployments, deployment)
	}

	return &DeploymentStore{
		deployments: deployments,
		mu:          sync.RWMutex{},
		db:          db,
	}, nil
}

// ListDeployments returns all deployments from the DB
func (ds *DeploymentStore) List() []DeploymentInfo {
	if ds == nil || ds.db == nil {
		return nil
	}
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	rows, err := ds.db.Query(`SELECT id, name, target_lxc, status, compose_yml FROM deployments`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make([]DeploymentInfo, 0)
	for rows.Next() {
		var d DeploymentInfo
		if err := rows.Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status, &d.ComposeYAML); err != nil {
			continue
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
	}
	return result
}

// GetDeployment retrieves a deployment by name
func (ds *DeploymentStore) Get(name string) (DeploymentInfo, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var d DeploymentInfo
	err := ds.db.QueryRow(`SELECT id, name, target_lxc, status, compose_yml FROM deployments WHERE name = ?`, name).
		Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status, &d.ComposeYAML)
	if err == sql.ErrNoRows {
		return DeploymentInfo{}, false
	}
	if err != nil {
		return DeploymentInfo{}, false
	}
	return d, true
}

// AddOrUpdateDeployment adds or updates a deployment in the DB
func (ds *DeploymentStore) Create(deployment DeploymentInfo) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Try update first (also update compose_yml)
	res, err := ds.db.Exec(`UPDATE deployments SET target_lxc = ?, compose_yml = ?, status = ? WHERE name = ?`,
		deployment.TargetLXC, deployment.ComposeYAML, deployment.Status, deployment.Name)
	if err != nil {
		return fmt.Errorf("failed to add or update deployment: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to add or update deployment: %w", err)
	}
	if ra > 0 {
		// update applied, don't insert
		return nil
	}

	id := uuid.New().String()
	_, err = ds.db.Exec(`INSERT INTO deployments (id, name, target_lxc, compose_yml, status) VALUES (?, ?, ?, ?, ?)`,
		id, deployment.Name, deployment.TargetLXC, deployment.ComposeYAML, deployment.Status)
	if err != nil {
		return fmt.Errorf("failed to add deployment: %w", err)
	}
	return nil
}

// Update Deployment updates an existing deployment in the DB
func (ds *DeploymentStore) Update(deployment DeploymentInfo) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, err := ds.db.Exec(`UPDATE deployments SET compose_yml = ?, status = ? WHERE name = ?`, deployment.ComposeYAML, deployment.Status, deployment.Name)
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
