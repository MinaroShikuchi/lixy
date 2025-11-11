// internal/client/pull_token_client.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

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
	ExpiresAt time.Time
	Registry  string
}

// PullTokenRequest represents a request for a pull token
type PullTokenRequest struct {
	AgentName string `json:"agent_name"`
	Registry  string `json:"registry"`
}

// PullTokenResponse represents the response from the controller
type PullTokenResponse struct {
	Success   bool   `json:"success"`
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"` // seconds
	Error     string `json:"error,omitempty"`
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

// GetPullToken requests a pull token for the specified registry
// Returns cached token if still valid, otherwise requests a new one
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

	token, expiresIn, err := c.requestToken(registry)
	if err != nil {
		return "", fmt.Errorf("failed to request pull token: %w", err)
	}

	// Cache the token with a safety margin (expire 1 minute early)
	expiresAt := time.Now().Add(time.Duration(expiresIn-60) * time.Second)

	c.mu.Lock()
	c.cachedTokens[registry] = &CachedToken{
		Token:     token,
		ExpiresAt: expiresAt,
		Registry:  registry,
	}
	c.mu.Unlock()

	c.logger.Info("Successfully obtained pull token",
		"registry", registry,
		"expires_at", expiresAt)

	return token, nil
}

// requestToken makes an HTTP request to the controller for a pull token
func (c *PullTokenClient) requestToken(registry string) (string, int, error) {
	// Get agent authentication token
	tokenData, err := c.tokenService.GetToken()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get agent token: %w", err)
	}

	reqBody := PullTokenRequest{
		AgentName: c.agentName,
		Registry:  registry,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/pull-token", c.controllerURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenData.Token))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("controller returned status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	var result PullTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("controller returned error: %s", result.Error)
	}

	return result.Token, result.ExpiresIn, nil
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
