// internal/domain/interfaces.go
package domain

import (
	"net/http"
)

type EndpointHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

type CommandHandler interface {
	HandleCommand(cmd Command) Response
}

type Command struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
