package controller

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type ControllerRouter struct {
	agentHandlers      *handlers.AgentHandlers
	deploymentHandlers *handlers.DeploymentHandlers
}

func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/register-agent", middlewares.LoggingMiddleware(ce.agentHandlers.RegisterAgentHandler))
	mux.HandleFunc("/api/tokens/registration", ce.agentHandlers.GenerateRegistrationTokenHandler)
	mux.HandleFunc("/api/unregister-agent", middlewares.AuthMiddleware(ce.agentHandlers.UnregisterAgentHandler))
	mux.HandleFunc("/api/save-agent", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.agentHandlers.SaveAgentHandler)))
	mux.HandleFunc("/api/deployments/", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.ListDeploymentsHandler)))
	mux.HandleFunc("/api/deployments/{name}", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.UpdateDeploymentStatus)))
	mux.HandleFunc("/admin/agents", middlewares.AuthMiddleware(ce.agentHandlers.ListAgentsHandler))

}

func NewControllerRouter(agentHandlers *handlers.AgentHandlers, deploymentHandlers *handlers.DeploymentHandlers) *ControllerRouter {
	return &ControllerRouter{
		agentHandlers:      agentHandlers,
		deploymentHandlers: deploymentHandlers,
	}
}
