// internal/store/_token_store.go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type TokenStore struct {
	db *sql.DB
}

// TokenData represents the authentication token data
type TokenData struct {
	Token         string    `json:"token"`
	IssuedAt      time.Time `json:"issued_at"`
	ControllerURL string    `json:"controller_url"`
}

// NewTokenStore creates a new -based token store
func NewTokenStore(db *sql.DB) (*TokenStore, error) {
	// Create tokens table if it doesn't exist
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS tokens (
            id TEXT PRIMARY KEY,
            token TEXT NOT NULL,
            issued_at INTEGER NOT NULL,
            controller_url TEXT NOT NULL
        )
    `)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tokens table: %w", err)
	}

	return &TokenStore{db: db}, nil
}

// Close closes the database connection
func (s *TokenStore) Close() error {
	return s.db.Close()
}

// Create saves an agent's token to the SQLite database
func (s *TokenStore) Create(token, controllerURL string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO tokens (id, token, issued_at, controller_url) VALUES (?, ?, ?, ?)",
		uuid.New().String(),
		token,
		time.Now().Unix(),
		controllerURL,
	)
	if err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}
	return nil
}

// Get retrieves a token from the SQLite database
func (s *TokenStore) Get() (TokenData, error) {
	row := s.db.QueryRow("SELECT token, issued_at, controller_url FROM tokens")

	var data TokenData
	var issuedAtUnix int64

	err := row.Scan(&data.Token, &issuedAtUnix, &data.ControllerURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return TokenData{}, fmt.Errorf("no token found for agent")
		}
		return TokenData{}, fmt.Errorf("failed to load token: %w", err)
	}

	data.IssuedAt = time.Unix(issuedAtUnix, 0)
	return data, nil
}

// DeleteToken removes a token from the database
func (s *TokenStore) Delete() error {
	_, err := s.db.Exec("DELETE FROM tokens")
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}
