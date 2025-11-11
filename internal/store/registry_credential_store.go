package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// RegistryCredential represents stored registry credentials
type RegistryCredential struct {
	ID           string    `json:"id"`
	RegistryType string    `json:"registry_type"` // "ghcr"
	Username     string    `json:"username"`
	Token        string    `json:"-"` // Never expose in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RegistryCredentialStore manages encrypted registry credentials
type RegistryCredentialStore struct {
	mu  sync.RWMutex
	db  *sql.DB
	gcm cipher.AEAD
}

// NewRegistryCredentialStore creates a new registry credential store
func NewRegistryCredentialStore(db *sql.DB, encryptionKey string) (*RegistryCredentialStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Create table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS registry_credentials (
			id TEXT PRIMARY KEY,
			registry_type TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL,
			token TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry_credentials table: %w", err)
	}

	// Create index on registry_type for faster lookups
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_registry_type 
		ON registry_credentials(registry_type)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	// Setup encryption
	var gcm cipher.AEAD
	if encryptionKey != "" {
		// Create AES cipher from key
		keyHash := sha256.Sum256([]byte(encryptionKey))
		block, err := aes.NewCipher(keyHash[:])
		if err != nil {
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}

		gcm, err = cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCM: %w", err)
		}
	} else {
		return nil, fmt.Errorf("encryption key is required for registry credential store")
	}

	return &RegistryCredentialStore{
		db:  db,
		gcm: gcm,
	}, nil
}

// StoreCredential stores encrypted registry credentials
func (s *RegistryCredentialStore) StoreCredential(registryType, username, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate inputs
	if registryType == "" {
		return fmt.Errorf("registry type cannot be empty")
	}
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Encrypt the token
	encryptedToken, err := s.encrypt(token)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Check if credential already exists
	var existingID string
	err = s.db.QueryRow(
		"SELECT id FROM registry_credentials WHERE registry_type = ?",
		registryType,
	).Scan(&existingID)

	now := time.Now().Unix()

	if err == sql.ErrNoRows {
		// Create new credential
		id := uuid.New().String()
		_, err = s.db.Exec(`
			INSERT INTO registry_credentials (id, registry_type, username, token, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, id, registryType, username, encryptedToken, now, now)
		if err != nil {
			return fmt.Errorf("failed to store credential: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing credential: %w", err)
	} else {
		// Update existing credential
		_, err = s.db.Exec(`
			UPDATE registry_credentials 
			SET username = ?, token = ?, updated_at = ?
			WHERE id = ?
		`, username, encryptedToken, now, existingID)
		if err != nil {
			return fmt.Errorf("failed to update credential: %w", err)
		}
	}

	return nil
}

// GetCredential retrieves and decrypts registry credentials
func (s *RegistryCredentialStore) GetCredential(registryType string) (*RegistryCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cred RegistryCredential
	var encryptedToken string
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, registry_type, username, token, created_at, updated_at
		FROM registry_credentials
		WHERE registry_type = ?
	`, registryType).Scan(
		&cred.ID,
		&cred.RegistryType,
		&cred.Username,
		&encryptedToken,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no credentials found for registry: %s", registryType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Decrypt the token
	decryptedToken, err := s.decrypt(encryptedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	cred.Token = decryptedToken
	cred.CreatedAt = time.Unix(createdAt, 0)
	cred.UpdatedAt = time.Unix(updatedAt, 0)

	return &cred, nil
}

// ListCredentials returns all stored credentials (without tokens for security)
func (s *RegistryCredentialStore) ListCredentials() ([]RegistryCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, registry_type, username, created_at, updated_at
		FROM registry_credentials
		ORDER BY registry_type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	defer rows.Close()

	var credentials []RegistryCredential
	for rows.Next() {
		var cred RegistryCredential
		var createdAt, updatedAt int64

		err := rows.Scan(
			&cred.ID,
			&cred.RegistryType,
			&cred.Username,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential: %w", err)
		}

		cred.CreatedAt = time.Unix(createdAt, 0)
		cred.UpdatedAt = time.Unix(updatedAt, 0)
		// Token is intentionally not included for security

		credentials = append(credentials, cred)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating credentials: %w", err)
	}

	return credentials, nil
}

// DeleteCredential removes registry credentials
func (s *RegistryCredentialStore) DeleteCredential(registryType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(
		"DELETE FROM registry_credentials WHERE registry_type = ?",
		registryType,
	)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no credentials found for registry: %s", registryType)
	}

	return nil
}

// UpdateCredential updates existing credentials
func (s *RegistryCredentialStore) UpdateCredential(registryType, username, token string) error {
	// UpdateCredential is the same as StoreCredential since we use INSERT OR REPLACE logic
	return s.StoreCredential(registryType, username, token)
}

// encrypt encrypts data using AES-GCM
func (s *RegistryCredentialStore) encrypt(plaintext string) (string, error) {
	if s.gcm == nil {
		return "", fmt.Errorf("encryption not initialized")
	}

	// Generate a random nonce
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-GCM
func (s *RegistryCredentialStore) decrypt(ciphertext string) (string, error) {
	if s.gcm == nil {
		return "", fmt.Errorf("encryption not initialized")
	}

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Check minimum length
	nonceSize := s.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and encrypted data
	nonce := data[:nonceSize]
	encryptedData := data[nonceSize:]

	// Decrypt
	plaintext, err := s.gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Close closes the database connection
func (s *RegistryCredentialStore) Close() error {
	return s.db.Close()
}
