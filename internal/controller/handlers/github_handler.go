package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/golang-jwt/jwt/v5"
)

type GitHubHandlers struct {
	logger      *slog.Logger
	configStore *store.ConfigStore
}

type GitHubConfigRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"` // For OAuth flow
	PrivateKey   string `json:"private_key,omitempty"`   // PEM-formatted private key for API calls
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

type GitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type GitHubUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// NewGitHubHandlers creates a new GitHub handlers instance
func NewGitHubHandlers(logger *slog.Logger, configStore *store.ConfigStore) *GitHubHandlers {
	return &GitHubHandlers{
		logger:      logger,
		configStore: configStore,
	}
}

// GitHubCallbackHandler handles the GitHub OAuth callback
func (gh *GitHubHandlers) GitHubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get GitHub configuration from secure storage
	clientID, clientSecret, privateKey, _ := gh.configStore.GetGitHubConfig()

	// Validate required configuration
	if clientID == "" || (privateKey == "" && clientSecret == "") {
		gh.logger.Error("GitHub App not configured", "missing", "client_id or auth credentials")
		http.Error(w, "GitHub App not configured", http.StatusInternalServerError)
		return
	}

	// Extract authorization code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Handle OAuth errors
	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		gh.logger.Error("GitHub OAuth error", "error", errorParam, "description", errorDesc)
		http.Error(w, fmt.Sprintf("GitHub OAuth error: %s", errorParam), http.StatusBadRequest)
		return
	}

	// Validate authorization code
	if code == "" {
		gh.logger.Error("Missing authorization code in GitHub callback")
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// TODO: Validate state parameter to prevent CSRF attacks
	// In a real implementation, you should store the state in a session/cache
	// and validate it matches what was sent in the authorization request
	if state == "" {
		gh.logger.Warn("Missing state parameter in GitHub callback")
	}

	// Exchange authorization code for access token
	accessToken, err := gh.exchangeCodeForToken(code)
	if err != nil {
		gh.logger.Error("Failed to exchange code for token", "error", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Get user information from GitHub
	user, err := gh.getUserInfo(accessToken)
	if err != nil {
		gh.logger.Error("Failed to get user info", "error", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	gh.logger.Info("GitHub OAuth successful", "user", user.Login, "id", user.ID)

	// TODO: Create or update user session/token in your system
	// This is where you would:
	// 1. Create a local user record or update existing one
	// 2. Generate a JWT or session token
	// 3. Store user permissions/roles
	// 4. Redirect to your application dashboard

	// For now, return success response with user info
	response := map[string]interface{}{
		"success": true,
		"message": "GitHub authentication successful",
		"user": map[string]interface{}{
			"id":    user.ID,
			"login": user.Login,
			"name":  user.Name,
			"email": user.Email,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		gh.logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GitHubAuthHandler initiates the GitHub OAuth flow
func (gh *GitHubHandlers) GitHubAuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get GitHub configuration from secure storage
	clientID, _, _, _ := gh.configStore.GetGitHubConfig()

	if clientID == "" {
		gh.logger.Error("GitHub OAuth not configured", "missing", "client_id")
		http.Error(w, "GitHub OAuth not configured", http.StatusInternalServerError)
		return
	}

	// Generate state parameter for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		gh.logger.Error("Failed to generate state", "error", err)
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	// TODO: Store state in session/cache for later validation

	// Build GitHub authorization URL
	authURL := gh.buildAuthURL(state)

	gh.logger.Info("Redirecting to GitHub for authorization", "state", state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// GitHubConfigHandler handles GitHub configuration updates
func (gh *GitHubHandlers) GitHubConfigHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		gh.getGitHubConfig(w, r)
	case http.MethodPost:
		gh.updateGitHubConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getGitHubConfig returns the current GitHub configuration (without secrets)
func (gh *GitHubHandlers) getGitHubConfig(w http.ResponseWriter, r *http.Request) {
	clientID, clientSecret, privateKey, _ := gh.configStore.GetGitHubConfig()

	config := map[string]interface{}{
		"client_id_configured":     clientID != "",
		"client_secret_configured": clientSecret != "",
		"private_key_configured":   privateKey != "",
		"redirect_uri":             gh.getDefaultRedirectURI(r),
		"auth_url":                 "/github/auth",
		"callback_url":             "/github/callback",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(config); err != nil {
		gh.logger.Error("Failed to encode config response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// updateGitHubConfig updates the GitHub OAuth configuration
func (gh *GitHubHandlers) updateGitHubConfig(w http.ResponseWriter, r *http.Request) {
	var req GitHubConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gh.logger.Error("Failed to decode config request", "error", err)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ClientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}
	if req.PrivateKey == "" {
		http.Error(w, "private_key is required", http.StatusBadRequest)
		return
	}

	// Get redirect URI (use provided or default)
	var redirectURI string
	if req.RedirectURI != "" {
		redirectURI = req.RedirectURI
	} else {
		redirectURI = gh.getDefaultRedirectURI(r)
	}

	// Store configuration securely
	if err := gh.configStore.SetGitHubConfig(req.ClientID, req.ClientSecret, req.PrivateKey, redirectURI); err != nil {
		gh.logger.Error("Failed to store GitHub config", "error", err)
		http.Error(w, "Failed to store configuration", http.StatusInternalServerError)
		return
	}

	gh.logger.Info("GitHub OAuth configuration updated",
		"client_id", req.ClientID,
		"redirect_uri", redirectURI)

	// Return success response
	response := map[string]interface{}{
		"success": true,
		"message": "GitHub OAuth configuration updated successfully",
		"config": map[string]interface{}{
			"client_id":    req.ClientID,
			"redirect_uri": redirectURI,
			"auth_url":     "/github/auth",
			"callback_url": "/github/callback",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		gh.logger.Error("Failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// getDefaultRedirectURI generates the default redirect URI based on the current request
func (gh *GitHubHandlers) getDefaultRedirectURI(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check for forwarded proto header (common in reverse proxy setups)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := r.Host
	if host == "" {
		host = "localhost:8080" // fallback
	}

	return fmt.Sprintf("%s://%s/github/callback", scheme, host)
}

// exchangeCodeForToken exchanges authorization code for access token using GitHub App
func (gh *GitHubHandlers) exchangeCodeForToken(code string) (string, error) {
	// Get GitHub configuration from secure storage
	clientID, clientSecret, privateKey, redirectURI := gh.configStore.GetGitHubConfig()

	// For GitHub Apps OAuth flow, we need a client_secret
	// If we don't have it stored but have a private key, we can fetch it using GitHub App API
	if clientSecret == "" && privateKey != "" {
		var err error
		clientSecret, err = gh.getOrCreateClientSecret(clientID, privateKey)
		if err != nil {
			return "", fmt.Errorf("failed to get client secret: %w", err)
		}
	}

	if clientSecret == "" {
		return "", fmt.Errorf("no client secret available for OAuth flow")
	}

	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	if redirectURI != "" {
		data.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed: %d %s", resp.StatusCode, string(body))
	}

	var oauthResp GitHubOAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&oauthResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if oauthResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return oauthResp.AccessToken, nil
}

// getOrCreateClientSecret handles getting the client secret for GitHub App OAuth
func (gh *GitHubHandlers) getOrCreateClientSecret(clientID, privateKey string) (string, error) {
	// Check if we have a client secret stored
	if clientSecret, exists := gh.configStore.Get("github.client_secret"); exists {
		return clientSecret, nil
	}

	// For GitHub Apps, you typically need to generate client secrets in the GitHub App settings
	// This is a placeholder - in production, you'd either:
	// 1. Require the user to provide both client_id AND client_secret from GitHub App settings
	// 2. Or use the Installation Access Token flow instead of user OAuth
	return "", fmt.Errorf("client_secret required for GitHub App OAuth - please provide both client_id and client_secret from your GitHub App settings")
}

// getUserInfo gets user information from GitHub API
func (gh *GitHubHandlers) getUserInfo(accessToken string) (*GitHubUser, error) {
	userURL := "https://api.github.com/user"

	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed: %d %s", resp.StatusCode, string(body))
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	return &user, nil
}

// buildAuthURL builds the GitHub authorization URL
func (gh *GitHubHandlers) buildAuthURL(state string) string {
	// Get GitHub configuration from secure storage
	clientID, _, _, redirectURI := gh.configStore.GetGitHubConfig()

	baseURL := "https://github.com/login/oauth/authorize"
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("scope", "read:user user:email")
	params.Set("state", state)
	if redirectURI != "" {
		params.Set("redirect_uri", redirectURI)
	}

	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

// generateRandomState generates a random state parameter for CSRF protection
func generateRandomState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// createGitHubAppJWT creates a JWT for GitHub App authentication
func (gh *GitHubHandlers) createGitHubAppJWT(clientID, privateKeyPEM string) (string, error) {
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

	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create JWT claims
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),                       // Issued at
		"exp": now.Add(10 * time.Minute).Unix(), // Expires in 10 minutes (GitHub's max)
		"iss": clientID,                         // Issuer (GitHub App ID)
	}

	// Create and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, nil
}
