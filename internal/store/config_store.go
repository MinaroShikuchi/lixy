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

	_ "github.com/mattn/go-sqlite3"
)

// ConfigStore manages encrypted configuration values
type ConfigStore struct {
	mu     sync.RWMutex
	db     *sql.DB
	gcm    cipher.AEAD
	config map[string]string // in-memory cache
}

// NewConfigStore creates a new ConfigStore with encryption
func NewConfigStore(db *sql.DB, encryptionKey string) (*ConfigStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Create table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			encrypted BOOLEAN DEFAULT TRUE,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create config table: %w", err)
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
	}

	store := &ConfigStore{
		db:     db,
		gcm:    gcm,
		config: make(map[string]string),
	}

	// Load existing configuration into memory
	if err := store.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return store, nil
}

// Set stores an encrypted configuration value
func (cs *ConfigStore) Set(key, value string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var encryptedValue string
	var encrypted bool

	if cs.gcm != nil {
		// Encrypt the value
		nonce := make([]byte, cs.gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return fmt.Errorf("failed to generate nonce: %w", err)
		}

		ciphertext := cs.gcm.Seal(nonce, nonce, []byte(value), nil)
		encryptedValue = base64.StdEncoding.EncodeToString(ciphertext)
		encrypted = true
	} else {
		// Store as plaintext if no encryption key
		encryptedValue = value
		encrypted = false
	}

	// Store in database
	_, err := cs.db.Exec(`
		INSERT OR REPLACE INTO config (key, value, encrypted, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, key, encryptedValue, encrypted)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	// Update in-memory cache
	cs.config[key] = value

	return nil
}

// Get retrieves and decrypts a configuration value
func (cs *ConfigStore) Get(key string) (string, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// Check in-memory cache first
	value, exists := cs.config[key]
	return value, exists
}

// GetAll returns all configuration keys (without values for security)
func (cs *ConfigStore) GetAll() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	keys := make([]string, 0, len(cs.config))
	for key := range cs.config {
		keys = append(keys, key)
	}
	return keys
}

// Delete removes a configuration value
func (cs *ConfigStore) Delete(key string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Remove from database
	_, err := cs.db.Exec(`DELETE FROM config WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	// Remove from in-memory cache
	delete(cs.config, key)

	return nil
}

// loadConfig loads all configuration from database into memory
func (cs *ConfigStore) loadConfig() error {
	rows, err := cs.db.Query(`SELECT key, value, encrypted FROM config`)
	if err != nil {
		return fmt.Errorf("failed to query config: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, encryptedValue string
		var encrypted bool

		if err := rows.Scan(&key, &encryptedValue, &encrypted); err != nil {
			return fmt.Errorf("failed to scan config row: %w", err)
		}

		var value string
		if encrypted && cs.gcm != nil {
			// Decrypt the value
			ciphertext, err := base64.StdEncoding.DecodeString(encryptedValue)
			if err != nil {
				return fmt.Errorf("failed to decode ciphertext for key %s: %w", key, err)
			}

			if len(ciphertext) < cs.gcm.NonceSize() {
				return fmt.Errorf("ciphertext too short for key %s", key)
			}

			nonce := ciphertext[:cs.gcm.NonceSize()]
			ciphertext = ciphertext[cs.gcm.NonceSize():]

			plaintext, err := cs.gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				return fmt.Errorf("failed to decrypt value for key %s: %w", key, err)
			}

			value = string(plaintext)
		} else {
			// Use plaintext value
			value = encryptedValue
		}

		cs.config[key] = value
	}

	return rows.Err()
}

// GitRepositoryConfig represents a Git repository configuration
type GitRepositoryConfig struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Branch    string `json:"branch"`
	LocalPath string `json:"local_path"`
	Token     string `json:"token,omitempty"` // Will be encrypted
}

// SetGitRepositoryConfig stores Git repository configuration securely
func (cs *ConfigStore) SetGitRepositoryConfig(name, url, branch, localPath, token string) error {
	keyPrefix := fmt.Sprintf("git_repo.%s", name)

	if err := cs.Set(fmt.Sprintf("%s.url", keyPrefix), url); err != nil {
		return fmt.Errorf("failed to store repository URL: %w", err)
	}
	if err := cs.Set(fmt.Sprintf("%s.branch", keyPrefix), branch); err != nil {
		return fmt.Errorf("failed to store repository branch: %w", err)
	}
	if err := cs.Set(fmt.Sprintf("%s.local_path", keyPrefix), localPath); err != nil {
		return fmt.Errorf("failed to store repository local path: %w", err)
	}
	if token != "" {
		if err := cs.Set(fmt.Sprintf("%s.token", keyPrefix), token); err != nil {
			return fmt.Errorf("failed to store repository token: %w", err)
		}
	}

	return nil
}

// GetGitRepositoryConfig retrieves Git repository configuration
func (cs *ConfigStore) GetGitRepositoryConfig(name string) (*GitRepositoryConfig, bool) {
	keyPrefix := fmt.Sprintf("git_repo.%s", name)

	url, urlExists := cs.Get(fmt.Sprintf("%s.url", keyPrefix))
	if !urlExists {
		return nil, false
	}

	branch, _ := cs.Get(fmt.Sprintf("%s.branch", keyPrefix))
	localPath, _ := cs.Get(fmt.Sprintf("%s.local_path", keyPrefix))
	token, _ := cs.Get(fmt.Sprintf("%s.token", keyPrefix))

	return &GitRepositoryConfig{
		Name:      name,
		URL:       url,
		Branch:    branch,
		LocalPath: localPath,
		Token:     token,
	}, true
}

// ListGitRepositories returns all configured Git repository names
func (cs *ConfigStore) ListGitRepositories() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	repoNames := make(map[string]bool)
	prefix := "git_repo."

	for key := range cs.config {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			// Extract repository name from key like "git_repo.lixy-cd.url"
			remaining := key[len(prefix):]
			if dotIndex := len(remaining); dotIndex > 0 {
				for i, c := range remaining {
					if c == '.' {
						dotIndex = i
						break
					}
				}
				if dotIndex > 0 && dotIndex < len(remaining) {
					repoName := remaining[:dotIndex]
					repoNames[repoName] = true
				}
			}
		}
	}

	names := make([]string, 0, len(repoNames))
	for name := range repoNames {
		names = append(names, name)
	}
	return names
}

// UpdateGitRepositoryToken updates the token for an existing repository
func (cs *ConfigStore) UpdateGitRepositoryToken(name, token string) error {
	keyPrefix := fmt.Sprintf("git_repo.%s", name)

	// Check if repository exists
	if _, exists := cs.Get(fmt.Sprintf("%s.url", keyPrefix)); !exists {
		return fmt.Errorf("repository %s not found", name)
	}

	return cs.Set(fmt.Sprintf("%s.token", keyPrefix), token)
}

// DeleteGitRepositoryConfig removes Git repository configuration
func (cs *ConfigStore) DeleteGitRepositoryConfig(name string) error {
	keyPrefix := fmt.Sprintf("git_repo.%s", name)

	// Delete all keys for this repository
	keys := []string{
		fmt.Sprintf("%s.url", keyPrefix),
		fmt.Sprintf("%s.branch", keyPrefix),
		fmt.Sprintf("%s.local_path", keyPrefix),
		fmt.Sprintf("%s.token", keyPrefix),
	}

	for _, key := range keys {
		if err := cs.Delete(key); err != nil {
			// Continue deleting other keys even if one fails
			continue
		}
	}

	return nil
}
