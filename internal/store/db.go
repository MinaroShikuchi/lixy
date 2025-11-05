package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB opens a SQLite database connection and prepares it for use.
// It handles directory creation, enables foreign keys, and applies any initial setup.
func InitDB(dbPath string) (*sql.DB, error) {
	// Create directory structure if path contains directories
	if dbDir := filepath.Dir(dbPath); dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// Enable foreign key enforcement for SQLite
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// CloseDB safely closes a database connection
func CloseDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
