package controller

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type ControllerRouter struct {
	agentHandlers *handlers.AgentHandlers
}

func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	// Create authenticated routes
	// authenticatedAPI := http.NewServeMux()
	// authenticatedAPI.HandleFunc("/api/status", agentHandlers.statusHandler)
	// mux.Handle("/api/", middlewares.AuthenticateAgent(authenticatedAPI))

	mux.HandleFunc("/api/register-agent", ce.agentHandlers.RegisterAgentHandler)
	mux.HandleFunc("/api/tokens/registration", ce.agentHandlers.GenerateRegistrationTokenHandler)
	mux.HandleFunc("/api/unregister-agent", middlewares.AuthMiddleware(ce.agentHandlers.UnregisterAgentHandler))
	mux.HandleFunc("/api/save-agent", middlewares.AuthMiddleware(ce.agentHandlers.SaveAgentHandler))
	// Admin routes for listing agents
	mux.HandleFunc("/admin/agents", middlewares.AuthMiddleware(ce.agentHandlers.ListAgentsHandler))

}

func NewControllerRouter(agentHandlers *handlers.AgentHandlers) *ControllerRouter {
	return &ControllerRouter{
		agentHandlers: agentHandlers,
	}
}
