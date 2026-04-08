package domain

import (
	"encoding/json"
	"net/http"
)

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey string

// Typed context keys for request context values.
const (
	AgentNameKey ctxKey = "agent_name"
	UserIDKey    ctxKey = "user_id"
	UsernameKey  ctxKey = "username"
	UserRoleKey  ctxKey = "user_role"
	TokenTypeKey ctxKey = "token_type"
)

// EndpointHandler defines an interface for registering HTTP routes.
type EndpointHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

// CommandHandler defines an interface for handling commands.
type CommandHandler interface {
	HandleCommand(cmd Command) Response
}

// Command represents a command to be executed.
type Command struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response represents the result of a command execution.
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
