package controller

import (
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/services"
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
	authHandlers         *handlers.AuthHandler
	userService          *services.UserService
}

func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	// Create auth middleware with user service
	authMiddleware := middlewares.AuthMiddleware(ce.userService)

	mux.HandleFunc("/health", ce.healthCheckHandler.HealthCheckHandler)
	mux.HandleFunc("/api/register-agent", middlewares.LoggingMiddleware(ce.agentHandlers.RegisterAgentHandler))
	mux.HandleFunc("/api/tokens/registration", ce.agentHandlers.GenerateRegistrationTokenHandler)
	mux.HandleFunc("/api/unregister-agent", authMiddleware(ce.agentHandlers.UnregisterAgentHandler))
	mux.HandleFunc("/api/save-agent", authMiddleware(middlewares.LoggingMiddleware(ce.agentHandlers.SaveAgentHandler)))
	mux.HandleFunc("/api/deployments", authMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.CreateOrListDeployments)))
	mux.HandleFunc("/api/deployments/{name}", authMiddleware(middlewares.LoggingMiddleware(ce.deploymentHandlers.HandleDeploymentByName)))
	mux.HandleFunc("/admin/agents", authMiddleware(middlewares.LoggingMiddleware(ce.agentHandlers.ListAgentsHandler)))
	mux.HandleFunc("/api/logs/stream", authMiddleware(ce.logHandlers.StreamLogsHandler))

	// GitHub OAuth routes
	mux.HandleFunc("/github/auth", middlewares.LoggingMiddleware(ce.githubHandlers.GitHubAuthHandler))
	mux.HandleFunc("/github/callback", middlewares.LoggingMiddleware(ce.githubHandlers.GitHubCallbackHandler))
	mux.HandleFunc("/api/github/config", authMiddleware(middlewares.LoggingMiddleware(ce.githubHandlers.GitHubConfigHandler)))

	// Dashboard routes
	mux.HandleFunc("/api/dashboard/overview", authMiddleware(ce.dashboardHandlers.GetOverview))
	mux.HandleFunc("/api/dashboard/stats", authMiddleware(ce.dashboardHandlers.GetStats))
	mux.HandleFunc("/api/dashboard/activity", authMiddleware(ce.dashboardHandlers.GetRecentActivity))

	// GitOps routes
	mux.HandleFunc("/api/gitops/repositories", authMiddleware(ce.gitopsHandlers.HandleRepositories))
	mux.HandleFunc("/api/gitops/repositories/{name}", authMiddleware(ce.gitopsHandlers.DeleteRepository))
	mux.HandleFunc("/api/gitops/repositories/{name}/token", authMiddleware(ce.gitopsHandlers.UpdateRepositoryToken))
	mux.HandleFunc("/api/gitops/repositories/{name}/environments", authMiddleware(ce.gitopsHandlers.GetRepositoryEnvironments))
	mux.HandleFunc("/api/gitops/sync", authMiddleware(ce.gitopsHandlers.SyncRepository))
	mux.HandleFunc("/api/gitops/reconcile", authMiddleware(ce.gitopsHandlers.ReconcileRepository))
	mux.HandleFunc("/api/gitops/reconcile/environment", authMiddleware(ce.gitopsHandlers.ReconcileEnvironment))
	mux.HandleFunc("/api/gitops/drift", authMiddleware(ce.gitopsHandlers.DetectDrift))
	mux.HandleFunc("/api/gitops/updates", authMiddleware(ce.gitopsHandlers.CheckUpdates))

	// Pull token routes (agent authentication required)
	mux.HandleFunc("/api/pull-token", authMiddleware(ce.pullTokenHandlers.RequestPullTokenHandler))

	// Registry credential management routes (admin authentication required)
	mux.HandleFunc("/api/registry/credentials", authMiddleware(ce.registryCredHandlers.HandleCredentials))

	// Authentication routes (public - no auth required)
	mux.HandleFunc("/api/auth/login", middlewares.LoggingMiddleware(ce.authHandlers.Login))

	// User authentication routes (accepts both user and agent tokens)
	mux.HandleFunc("/api/auth/logout", authMiddleware(ce.authHandlers.Logout))
	mux.HandleFunc("/api/auth/me", authMiddleware(ce.authHandlers.GetCurrentUser))
	mux.HandleFunc("/api/auth/change-password", authMiddleware(ce.authHandlers.ChangePassword))

	// User management routes (admin only - role checked in handlers)
	mux.HandleFunc("/api/users", authMiddleware(ce.authHandlers.HandleUsers))
	mux.HandleFunc("/api/users/{id}", authMiddleware(ce.authHandlers.HandleUserByID))
	mux.HandleFunc("/api/users/{id}/password", authMiddleware(ce.authHandlers.ResetPassword))
}

func NewControllerRouter(agentHandlers *handlers.AgentHandlers, deploymentHandlers *handlers.DeploymentHandlers, healthcheckHandlers *handlers.HealthCheckHandler, logHandler *handlers.LogHandlers, githubHandlers *handlers.GitHubHandlers, dashboardHandlers *handlers.DashboardHandlers, gitopsHandlers *handlers.GitOpsHandlers, pullTokenHandlers *handlers.PullTokenHandlers, registryCredHandlers *handlers.RegistryCredentialHandlers, authHandlers *handlers.AuthHandler, userService *services.UserService) *ControllerRouter {
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
		authHandlers:         authHandlers,
		userService:          userService,
	}
}
