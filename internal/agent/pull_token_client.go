package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

// PullTokenClient handles requesting and caching pull tokens from the controller
type PullTokenClient struct {
	logger        *slog.Logger
	controllerURL string
	agentName     string
	tokenService  *services.TokenService
	client        *http.Client

	// Token cache
	mu           sync.RWMutex
	cachedTokens map[string]*CachedToken
}

// CachedToken represents a cached pull token with expiration
type CachedToken struct {
	Token     string
	Username  string
	ExpiresAt time.Time
	Registry  string
}

// NewPullTokenClient creates a new pull token client
func NewPullTokenClient(logger *slog.Logger, controllerURL, agentName string, tokenService *services.TokenService) *PullTokenClient {
	return &PullTokenClient{
		logger:        logger,
		controllerURL: controllerURL,
		agentName:     agentName,
		tokenService:  tokenService,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cachedTokens: make(map[string]*CachedToken),
	}
}

// GetPullTokenWithUsername requests a pull token for the specified registry
// Returns both token and username (cached if still valid, otherwise requests new)
func (c *PullTokenClient) GetPullTokenWithUsername(registry string) (token, username string, err error) {
	// Check cache first
	c.mu.RLock()
	cached, exists := c.cachedTokens[registry]
	c.mu.RUnlock()

	if exists && time.Now().Before(cached.ExpiresAt) {
		c.logger.Debug("Using cached pull token",
			"registry", registry,
			"expires_at", cached.ExpiresAt)
		return cached.Token, cached.Username, nil
	}

	// Request new token from controller
	c.logger.Info("Requesting pull token from controller",
		"registry", registry,
		"agent", c.agentName)

	tokenData, err := c.requestToken(registry)
	if err != nil {
		return "", "", fmt.Errorf("failed to request pull token: %w", err)
	}

	// Cache the token with a safety margin (expire 1 minute early)
	expiresAt := time.Now().Add(time.Duration(tokenData.ExpiresIn-60) * time.Second)

	c.mu.Lock()
	c.cachedTokens[registry] = &CachedToken{
		Token:     tokenData.Token,
		Username:  tokenData.Username,
		ExpiresAt: expiresAt,
		Registry:  registry,
	}
	c.mu.Unlock()

	c.logger.Info("Successfully obtained pull token",
		"token", tokenData.Token,
		"username", tokenData.Username,
		"registry", registry,
		"expires_at", expiresAt)

	return tokenData.Token, tokenData.Username, nil
}

// GetPullToken requests a pull token for the specified registry
// Returns cached token if still valid, otherwise requests a new one
// Deprecated: Use GetPullTokenWithUsername to also get the username
func (c *PullTokenClient) GetPullToken(registry string) (string, error) {
	// Check cache first
	c.mu.RLock()
	cached, exists := c.cachedTokens[registry]
	c.mu.RUnlock()

	if exists && time.Now().Before(cached.ExpiresAt) {
		c.logger.Debug("Using cached pull token",
			"registry", registry,
			"expires_at", cached.ExpiresAt)
		return cached.Token, nil
	}

	// Request new token from controller
	c.logger.Info("Requesting pull token from controller",
		"registry", registry,
		"agent", c.agentName)

	tokenData, err := c.requestToken(registry)
	if err != nil {
		return "", fmt.Errorf("failed to request pull token: %w", err)
	}

	// Cache the token with a safety margin (expire 1 minute early)
	expiresAt := time.Now().Add(time.Duration(tokenData.ExpiresIn-60) * time.Second)

	c.mu.Lock()
	c.cachedTokens[registry] = &CachedToken{
		Token:     tokenData.Token,
		Username:  tokenData.Username,
		ExpiresAt: expiresAt,
		Registry:  registry,
	}
	c.mu.Unlock()

	c.logger.Info("Successfully obtained pull token",
		"username", tokenData.Username,
		"registry", registry,
		"expires_at", expiresAt)

	return tokenData.Token, nil
}

// requestToken makes an HTTP request to the controller for a pull token
func (c *PullTokenClient) requestToken(registry string) (*domain.PullTokenAPIResponse, error) {
	// Get agent authentication token
	tokenData, err := c.tokenService.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent token: %w", err)
	}

	reqBody := domain.PullTokenRequest{
		AgentName: c.agentName,
		Registry:  registry,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/pull-token", c.controllerURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenData.Token))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result domain.PullTokenAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned error: %s", result.Error)
	}

	return &result, nil
}

// ClearCache clears all cached tokens
func (c *PullTokenClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedTokens = make(map[string]*CachedToken)
	c.logger.Info("Cleared pull token cache")
}

// ClearTokenForRegistry clears the cached token for a specific registry
func (c *PullTokenClient) ClearTokenForRegistry(registry string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cachedTokens, registry)
	c.logger.Info("Cleared cached token for registry", "registry", registry)
}

// GetCachedTokenInfo returns information about cached tokens (for debugging)
func (c *PullTokenClient) GetCachedTokenInfo() map[string]time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := make(map[string]time.Time)
	for registry, cached := range c.cachedTokens {
		info[registry] = cached.ExpiresAt
	}
	return info
}
