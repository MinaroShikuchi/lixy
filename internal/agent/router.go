package agent

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/agent/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type AgentRouter struct {
}

func (ce *AgentRouter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", middlewares.LoggingMiddleware(handlers.HealthCheckHandler))
}

func NewAgentRouter() *AgentRouter {
	return &AgentRouter{}
}
