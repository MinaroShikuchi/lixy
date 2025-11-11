package controller

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

type ControllerRouter struct {
	agentHandlers        *handlers.AgentHandlers
	deploymentHandlers   *handlers.DeploymentHandlers
	healthCheckHandler   *handlers.HealthCheckHandler
	logHandlers          *handlers.LogHandlers
	githubHandlers       *handlers.GitHubHandlers
	dashboardHandlers    *handlers.DashboardHandlers
	gitopsHandlers       *handlers.GitOpsHandlers
	pullTokenHandlers    *handlers.PullTokenHandlers
	registryCredHandlers *handlers.RegistryCredentialHandlers
}

func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", ce.healthCheckHandler.HealthCheckHandler)
	mux.HandleFunc("/api/register-agent", middlewares.LoggingMiddleware(ce.agentHandlers.RegisterAgentHandler))
	mux.HandleFunc("/api/tokens/registration", ce.agentHandlers.GenerateRegistrationTokenHandler)
	mux.HandleFunc("/api/unregister-agent", middlewares.AuthMiddleware(ce.agentHandlers.UnregisterAgentHandler))
	mux.HandleFunc("/api/save-agent", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.agentHandlers.SaveAgentHandler)))
	mux.HandleFunc("/api/deployments", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.CreateOrListDeployments)))
	mux.HandleFunc("/api/deployments/{name}", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.UpdateDeploymentStatus)))
	mux.HandleFunc("/admin/agents", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.agentHandlers.ListAgentsHandler)))
	mux.HandleFunc("/api/logs/stream", middlewares.AuthMiddleware(ce.logHandlers.StreamLogsHandler))

	// GitHub OAuth routes
	mux.HandleFunc("/github/auth", middlewares.LoggingMiddleware(ce.githubHandlers.GitHubAuthHandler))
	mux.HandleFunc("/github/callback", middlewares.LoggingMiddleware(ce.githubHandlers.GitHubCallbackHandler))
	mux.HandleFunc("/api/github/config", middlewares.AuthMiddleware(middlewares.LoggingMiddleware(ce.githubHandlers.GitHubConfigHandler)))

	// Dashboard routes
	mux.HandleFunc("/api/dashboard/overview", middlewares.AuthMiddleware(ce.dashboardHandlers.GetOverview))
	mux.HandleFunc("/api/dashboard/stats", middlewares.AuthMiddleware(ce.dashboardHandlers.GetStats))
	mux.HandleFunc("/api/dashboard/activity", middlewares.AuthMiddleware(ce.dashboardHandlers.GetRecentActivity))

	// GitOps routes
	mux.HandleFunc("/api/gitops/repositories", middlewares.AuthMiddleware(ce.gitopsHandlers.HandleRepositories))
	mux.HandleFunc("/api/gitops/repositories/{name}", middlewares.AuthMiddleware(ce.gitopsHandlers.DeleteRepository))
	mux.HandleFunc("/api/gitops/repositories/{name}/token", middlewares.AuthMiddleware(ce.gitopsHandlers.UpdateRepositoryToken))
	mux.HandleFunc("/api/gitops/repositories/{name}/environments", middlewares.AuthMiddleware(ce.gitopsHandlers.GetRepositoryEnvironments))
	mux.HandleFunc("/api/gitops/sync", middlewares.AuthMiddleware(ce.gitopsHandlers.SyncRepository))
	mux.HandleFunc("/api/gitops/reconcile", middlewares.AuthMiddleware(ce.gitopsHandlers.ReconcileRepository))
	mux.HandleFunc("/api/gitops/reconcile/environment", middlewares.AuthMiddleware(ce.gitopsHandlers.ReconcileEnvironment))
	mux.HandleFunc("/api/gitops/drift", middlewares.AuthMiddleware(ce.gitopsHandlers.DetectDrift))
	mux.HandleFunc("/api/gitops/updates", middlewares.AuthMiddleware(ce.gitopsHandlers.CheckUpdates))

	// Pull token routes (agent authentication required)
	mux.HandleFunc("/api/pull-token", middlewares.AuthMiddleware(ce.pullTokenHandlers.RequestPullTokenHandler))

	// Registry credential management routes (admin authentication required)
	mux.HandleFunc("/api/registry/credentials", middlewares.AuthMiddleware(ce.registryCredHandlers.HandleCredentials))
}

func NewControllerRouter(agentHandlers *handlers.AgentHandlers, deploymentHandlers *handlers.DeploymentHandlers, healthcheckHandlers *handlers.HealthCheckHandler, logHandler *handlers.LogHandlers, githubHandlers *handlers.GitHubHandlers, dashboardHandlers *handlers.DashboardHandlers, gitopsHandlers *handlers.GitOpsHandlers, pullTokenHandlers *handlers.PullTokenHandlers, registryCredHandlers *handlers.RegistryCredentialHandlers) *ControllerRouter {
	return &ControllerRouter{
		agentHandlers:        agentHandlers,
		deploymentHandlers:   deploymentHandlers,
		healthCheckHandler:   healthcheckHandlers,
		logHandlers:          logHandler,
		githubHandlers:       githubHandlers,
		dashboardHandlers:    dashboardHandlers,
		gitopsHandlers:       gitopsHandlers,
		pullTokenHandlers:    pullTokenHandlers,
		registryCredHandlers: registryCredHandlers,
	}
}
