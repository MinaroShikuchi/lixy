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
