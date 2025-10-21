package agent

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/agent/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type AgentRouter struct {
	// Agent-specific fields
}

func (ce *AgentRouter) RegisterRoutes(mux *http.ServeMux) {
	// Add handlers with logging middleware
	mux.HandleFunc("/healthz", middlewares.LoggingMiddleware(handlers.HealthCheckHandler))
	mux.HandleFunc("/deploy", middlewares.LoggingMiddleware(handlers.DeployHandler))
	mux.HandleFunc("/update", middlewares.LoggingMiddleware(handlers.UpdateDeploymentHandler))
}

func NewAgentRouter() *AgentRouter {
	return &AgentRouter{}
}
