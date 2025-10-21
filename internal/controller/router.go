package controller

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type ControllerRouter struct {
}

func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	// Register controller-specific routes
	// Create authenticated routes
	authenticatedAPI := http.NewServeMux()
	// authenticatedAPI.HandleFunc("/api/status", statusHandler)
	// Add authentication middleware for protected routes
	mux.Handle("/api/", middlewares.AuthenticateAgent(authenticatedAPI))

	// Add agent registration endpoints
	mux.HandleFunc("/api/register-agent", handlers.RegisterAgentHandler)
	mux.HandleFunc("/api/tokens/registration", handlers.GenerateRegistrationTokenHandler)
	// Admin routes for listing agents
	mux.HandleFunc("/admin/agents", handlers.ListAgentsHandler)
}

func NewControllerRouter() *ControllerRouter {
	return &ControllerRouter{}
}
