// internal/domain/interfaces.go
package domain

import (
	"encoding/json"
	"net/http"
)

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey string

const AgentNameKey ctxKey = "agent_name"

type EndpointHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

type CommandHandler interface {
	HandleCommand(cmd Command) Response
}

type Command struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type DeploymentRequest struct {
	Name        string            `json:"name"`
	ComposeYAML []byte            `json:"composeYAML"`
	EnvVars     map[string]string `json:"envVars,omitempty"`
}

type DeploymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

type DeploymentDto struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TargetLXC   string `json:"target_lxc"`
	Status      string `json:"status"`
	ComposeYAML []byte `json:"compose_yaml"`
}

// SystemInfo holds agent environment information
type SystemInfo struct {
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
}

// struct for registration response
type RegistrationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

// struct for decoding registration request
type RegistrationRequest struct {
	Hostname string `json:"hostname"`
	Token    string `json:"token"`
}

// struct for decoding registration request
type UnregistrationRequest struct {
}

// struct for decoding save agent request
type SaveAgentRequest struct {
	Version string `json:"version"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

// struct for decoding save agent response
type SaveAgentResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
}

type DeploymentStatusUpdate struct {
	Status string `json:"status"`
}

type CreateDeploymentRequest struct {
	Name        string `json:"name"`
	ComposeYAML []byte `json:"compose_yaml"`
	TargetLXC   string `json:"target_lxc"`
}

// Pull token request/response types

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
