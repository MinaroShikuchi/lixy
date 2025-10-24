package store

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// DeploymentInfo represents information about a deployment with docker compose
type DeploymentInfo struct {
	ID        int64
	Name      string
	TargetLXC string
	Status    string
}

// DeploymentStore manages deployments persisted in SQLite
type DeploymentStore struct {
	deployments []DeploymentInfo
	mu          sync.RWMutex
	db          *sql.DB
}

// NewDeploymentStore creates a new DeploymentStore backed by SQLite
func NewDeploymentStore(db *sql.DB) (*DeploymentStore, error) {
	// Enable foreign key enforcement for SQLite
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create deployments table with foreign key constraint on target_lxc
	// Note: this references the lxc_targets table — ensure that table exists with a "name" column.
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS deployments (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            target_lxc TEXT NOT NULL,
            status TEXT NOT NULL,
            FOREIGN KEY (target_lxc) REFERENCES lxc_targets(name) ON DELETE RESTRICT ON UPDATE CASCADE
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
func (ds *DeploymentStore) ListDeployments() []DeploymentInfo {
	if ds == nil || ds.db == nil {
		log.Printf("ListDeployments: deployment store or database is nil")
		return nil
	}
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	rows, err := ds.db.Query(`SELECT id, name, target_lxc, status FROM deployments`)
	if err != nil {
		log.Printf("ListDeployments query error: %v", err)
		return nil
	}
	defer rows.Close()

	result := make([]DeploymentInfo, 0)
	for rows.Next() {
		var d DeploymentInfo
		if err := rows.Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status); err != nil {
			log.Printf("ListDeployments scan error: %v", err)
			continue
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("ListDeployments rows error: %v", err)
	}
	return result
}

// GetDeployment retrieves a deployment by name
func (ds *DeploymentStore) GetDeployment(name string) (DeploymentInfo, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var d DeploymentInfo
	err := ds.db.QueryRow(`SELECT id, name, target_lxc, status FROM deployments WHERE name = ?`, name).
		Scan(&d.ID, &d.Name, &d.TargetLXC, &d.Status)
	if err == sql.ErrNoRows {
		return DeploymentInfo{}, false
	}
	if err != nil {
		log.Printf("GetDeployment error: %v", err)
		return DeploymentInfo{}, false
	}
	return d, true
}

// AddOrUpdateDeployment adds or updates a deployment in the DB
func (ds *DeploymentStore) AddOrUpdateDeployment(deployment DeploymentInfo) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Try update first
	res, err := ds.db.Exec(`UPDATE deployments SET target_lxc = ?, status = ? WHERE name = ?`,
		deployment.TargetLXC, deployment.Status, deployment.Name)
	if err != nil {
		log.Printf("AddOrUpdateDeployment update error: %v", err)
		return
	}
	ra, err := res.RowsAffected()
	if err != nil {
		log.Printf("AddOrUpdateDeployment RowsAffected error: %v", err)
		return
	}
	if ra > 0 {
		return
	}

	// Insert if update didn't affect any row
	res, err = ds.db.Exec(`INSERT INTO deployments (name, target_lxc, status) VALUES (?, ?, ?)`,
		deployment.Name, deployment.TargetLXC, deployment.Status)
	if err != nil {
		log.Printf("AddOrUpdateDeployment insert error: %v", err)
		return
	}
	id, err := res.LastInsertId()
	if err == nil {
		deployment.ID = id
	}
}

// DeleteDeployment removes a deployment by name from the DB
func (ds *DeploymentStore) DeleteDeployment(name string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	_, err := ds.db.Exec(`DELETE FROM deployments WHERE name = ?`, name)
	if err != nil {
		log.Printf("DeleteDeployment error: %v", err)
		return err
	}
	return nil
}
