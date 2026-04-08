package domain

import "time"

// TokenData represents the authentication token data
type TokenData struct {
	Token         string    `json:"token"`
	IssuedAt      time.Time `json:"issued_at"`
	ControllerURL string    `json:"controller_url"`
}

// PullTokenRequest represents a request for a pull token
type PullTokenRequest struct {
	AgentName string `json:"agent_name"`
	Registry  string `json:"registry"`
}

// PullTokenAPIResponse represents the API response for pull token requests
// Used for HTTP communication between controller and agents
type PullTokenAPIResponse struct {
	Success   bool   `json:"success"`
	Token     string `json:"token,omitempty"`
	Username  string `json:"username,omitempty"`
	Registry  string `json:"registry,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"` // seconds
	ExpiresAt string `json:"expires_at,omitempty"` // RFC3339 format
	Error     string `json:"error,omitempty"`
}

// TokenRepository defines the persistence interface for tokens
type TokenRepository interface {
	Get() (TokenData, error)
	Create(token, controllerURL string) error
}
