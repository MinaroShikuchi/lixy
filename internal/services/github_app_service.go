package services

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/golang-jwt/jwt/v5"
)

// GitHubInstallation represents a GitHub App installation
type GitHubInstallation struct {
	ID          int64  `json:"id"`
	Account     string `json:"account_login"`
	AccountType string `json:"account_type"`
}

// GitHubInstallationToken represents an installation access token
type GitHubInstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GitHubAppService manages GitHub App authentication and token generation
type GitHubAppService struct {
	logger      *slog.Logger
	configStore *store.ConfigStore

	// Token cache
	mu          sync.RWMutex
	cachedToken *GitHubInstallationToken

	httpClient *http.Client
}

// NewGitHubAppService creates a new GitHub App service
func NewGitHubAppService(logger *slog.Logger, configStore *store.ConfigStore) *GitHubAppService {
	return &GitHubAppService{
		logger:      logger,
		configStore: configStore,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetInstallationToken returns a valid installation access token, using cache if available
func (s *GitHubAppService) GetInstallationToken() (*GitHubInstallationToken, error) {
	// Check cache first
	s.mu.RLock()
	if s.cachedToken != nil && time.Now().Before(s.cachedToken.ExpiresAt.Add(-5*time.Minute)) {
		token := s.cachedToken
		s.mu.RUnlock()
		s.logger.Debug("Using cached GitHub installation token",
			"expires_at", token.ExpiresAt)
		return token, nil
	}
	s.mu.RUnlock()

	// Generate new token
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cachedToken != nil && time.Now().Before(s.cachedToken.ExpiresAt.Add(-5*time.Minute)) {
		return s.cachedToken, nil
	}

	// Get GitHub App configuration
	appID, _, privateKeyPEM, _ := s.configStore.GetGitHubConfig()
	if appID == "" || privateKeyPEM == "" {
		return nil, fmt.Errorf("GitHub App not configured - please configure App ID and private key")
	}

	// Generate JWT for GitHub App authentication
	jwt, err := s.createGitHubAppJWT(appID, privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App JWT: %w", err)
	}

	// Get installation ID (discover or use cached)
	installationID, err := s.getInstallationID(jwt)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation ID: %w", err)
	}

	// Request installation access token
	token, err := s.requestInstallationToken(jwt, installationID)
	if err != nil {
		return nil, fmt.Errorf("failed to request installation token: %w", err)
	}

	// Cache the token
	s.cachedToken = token

	s.logger.Info("Generated new GitHub installation token",
		"installation_id", installationID,
		"expires_at", token.ExpiresAt)

	return token, nil
}

// createGitHubAppJWT creates a JWT for GitHub App authentication
func (s *GitHubAppService) createGitHubAppJWT(appID, privateKeyPEM string) (string, error) {
	// Parse the PEM private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM private key")
	}

	var privateKey *rsa.PrivateKey
	var err error

	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse PKCS1 private key: %w", err)
		}
	case "PRIVATE KEY":
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		var ok bool
		privateKey, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("private key is not RSA")
		}
	default:
		return "", fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	// Create JWT claims
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),                       // Issued at
		"exp": now.Add(10 * time.Minute).Unix(), // Expires in 10 minutes (GitHub's max)
		"iss": appID,                            // Issuer (GitHub App ID)
	}

	// Create and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, nil
}

// getInstallationID discovers or retrieves the GitHub App installation ID
func (s *GitHubAppService) getInstallationID(jwtToken string) (int64, error) {
	// Check if we have a cached installation ID
	if installationIDStr, exists := s.configStore.Get("github.installation_id"); exists {
		var installationID int64
		if _, err := fmt.Sscanf(installationIDStr, "%d", &installationID); err == nil {
			return installationID, nil
		}
	}

	// Discover installations
	url := "https://api.github.com/app/installations"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get installations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to get installations: %d %s", resp.StatusCode, string(body))
	}

	var installations []GitHubInstallation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return 0, fmt.Errorf("failed to decode installations: %w", err)
	}

	if len(installations) == 0 {
		return 0, fmt.Errorf("no GitHub App installations found - please install the app on your organization/repositories")
	}

	// Use the first installation (most common case)
	installation := installations[0]

	// Cache the installation ID
	if err := s.configStore.Set("github.installation_id", fmt.Sprintf("%d", installation.ID)); err != nil {
		s.logger.Warn("Failed to cache installation ID", "error", err)
	}

	s.logger.Info("Discovered GitHub App installation",
		"installation_id", installation.ID,
		"account", installation.Account,
		"type", installation.AccountType)

	return installation.ID, nil
}

// requestInstallationToken requests an installation access token from GitHub
func (s *GitHubAppService) requestInstallationToken(jwtToken string, installationID int64) (*GitHubInstallationToken, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtToken))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create installation token: %d %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &GitHubInstallationToken{
		Token:     tokenResp.Token,
		ExpiresAt: tokenResp.ExpiresAt,
	}, nil
}

// InvalidateCache clears the cached installation token
func (s *GitHubAppService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedToken = nil
	s.logger.Info("GitHub installation token cache invalidated")
}

// IsConfigured checks if GitHub App is properly configured
func (s *GitHubAppService) IsConfigured() bool {
	appID, _, privateKey, _ := s.configStore.GetGitHubConfig()
	return appID != "" && privateKey != ""
}

// GetAppID returns the configured GitHub App ID
func (s *GitHubAppService) GetAppID() (string, error) {
	appID, _, _, _ := s.configStore.GetGitHubConfig()
	if appID == "" {
		return "", fmt.Errorf("GitHub App ID not configured")
	}
	return appID, nil
}
