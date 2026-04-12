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
	dashboardHandlers    *handlers.DashboardHandlers
	gitopsHandlers       *handlers.GitOpsHandlers
	pullTokenHandlers    *handlers.PullTokenHandlers
	registryCredHandlers *handlers.RegistryCredentialHandlers
	harborHandlers       *handlers.HarborHandlers
	authHandlers         *handlers.AuthHandler
	userService          *services.UserService
	authService          *services.AuthService
}

// RouterDeps groups all dependencies needed to construct a ControllerRouter.
type RouterDeps struct {
	AgentHandlers        *handlers.AgentHandlers
	DeploymentHandlers   *handlers.DeploymentHandlers
	HealthCheckHandler   *handlers.HealthCheckHandler
	LogHandlers          *handlers.LogHandlers
	DashboardHandlers    *handlers.DashboardHandlers
	GitOpsHandlers       *handlers.GitOpsHandlers
	PullTokenHandlers    *handlers.PullTokenHandlers
	RegistryCredHandlers *handlers.RegistryCredentialHandlers
	HarborHandlers       *handlers.HarborHandlers
	AuthHandlers         *handlers.AuthHandler
	AuthService          *services.AuthService
	UserService          *services.UserService
}

// NewControllerRouter creates a ControllerRouter from a RouterDeps struct.
func NewControllerRouter(deps RouterDeps) *ControllerRouter {
	return &ControllerRouter{
		agentHandlers:        deps.AgentHandlers,
		deploymentHandlers:   deps.DeploymentHandlers,
		healthCheckHandler:   deps.HealthCheckHandler,
		logHandlers:          deps.LogHandlers,
		dashboardHandlers:    deps.DashboardHandlers,
		gitopsHandlers:       deps.GitOpsHandlers,
		pullTokenHandlers:    deps.PullTokenHandlers,
		registryCredHandlers: deps.RegistryCredHandlers,
		harborHandlers:       deps.HarborHandlers,
		authHandlers:         deps.AuthHandlers,
		userService:          deps.UserService,
		authService:          deps.AuthService,
	}
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (ce *ControllerRouter) RegisterRoutes(mux *http.ServeMux) {
	authMiddleware := middlewares.AuthMiddleware(ce.userService, ce.authService)

	ce.registerHealthRoutes(mux)
	ce.registerAgentRoutes(mux, authMiddleware)
	ce.registerDeploymentRoutes(mux, authMiddleware)
	ce.registerLogRoutes(mux, authMiddleware)
	ce.registerDashboardRoutes(mux, authMiddleware)
	ce.registerGitOpsRoutes(mux, authMiddleware)
	ce.registerPullTokenRoutes(mux, authMiddleware)
	ce.registerRegistryRoutes(mux, authMiddleware)
	ce.registerHarborRoutes(mux, authMiddleware)
	ce.registerAuthRoutes(mux, authMiddleware)
}

// authMiddlewareFunc is the type returned by middlewares.AuthMiddleware.
type authMiddlewareFunc = func(http.HandlerFunc) http.HandlerFunc

// registerHealthRoutes registers health-check endpoints (no auth).
func (ce *ControllerRouter) registerHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", ce.healthCheckHandler.HealthCheckHandler)
}

// registerAgentRoutes registers agent management endpoints.
func (ce *ControllerRouter) registerAgentRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/register-agent", middlewares.LoggingMiddleware(ce.agentHandlers.RegisterAgentHandler))
	mux.HandleFunc("/api/tokens/registration", ce.agentHandlers.GenerateRegistrationTokenHandler)
	mux.HandleFunc("/api/unregister-agent", auth(ce.agentHandlers.UnregisterAgentHandler))
	mux.HandleFunc("/api/save-agent", auth(middlewares.LoggingMiddleware(ce.agentHandlers.SaveAgentHandler)))
	mux.HandleFunc("/admin/agents", auth(middlewares.LoggingMiddleware(ce.agentHandlers.ListAgentsHandler)))
}

// registerDeploymentRoutes registers deployment CRUD endpoints.
func (ce *ControllerRouter) registerDeploymentRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/deployments", auth(middlewares.LoggingMiddleware(ce.deploymentHandlers.CreateOrListDeployments)))
	mux.HandleFunc("/api/deployments/{name}", auth(middlewares.LoggingMiddleware(ce.deploymentHandlers.HandleDeploymentByName)))
}

// registerLogRoutes registers log streaming endpoints.
func (ce *ControllerRouter) registerLogRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/logs/stream", auth(ce.logHandlers.StreamLogsHandler))
}

// registerDashboardRoutes registers dashboard data endpoints.
func (ce *ControllerRouter) registerDashboardRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/dashboard/overview", auth(ce.dashboardHandlers.GetOverview))
	mux.HandleFunc("/api/dashboard/stats", auth(ce.dashboardHandlers.GetStats))
	mux.HandleFunc("/api/dashboard/activity", auth(ce.dashboardHandlers.GetRecentActivity))
}

// registerGitOpsRoutes registers GitOps repository and reconciliation endpoints.
func (ce *ControllerRouter) registerGitOpsRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/gitops/repositories", auth(ce.gitopsHandlers.HandleRepositories))
	mux.HandleFunc("/api/gitops/repositories/{name}", auth(ce.gitopsHandlers.DeleteRepository))
	mux.HandleFunc("/api/gitops/repositories/{name}/token", auth(ce.gitopsHandlers.UpdateRepositoryToken))
	mux.HandleFunc("/api/gitops/repositories/{name}/environments", auth(ce.gitopsHandlers.GetRepositoryEnvironments))
	mux.HandleFunc("/api/gitops/sync", auth(ce.gitopsHandlers.SyncRepository))
	mux.HandleFunc("/api/gitops/reconcile", auth(ce.gitopsHandlers.ReconcileRepository))
	mux.HandleFunc("/api/gitops/reconcile/environment", auth(ce.gitopsHandlers.ReconcileEnvironment))
	mux.HandleFunc("/api/gitops/drift", auth(ce.gitopsHandlers.DetectDrift))
	mux.HandleFunc("/api/gitops/updates", auth(ce.gitopsHandlers.CheckUpdates))
}

// registerPullTokenRoutes registers pull-token endpoints.
func (ce *ControllerRouter) registerPullTokenRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/pull-token", auth(ce.pullTokenHandlers.RequestPullTokenHandler))
}

// registerRegistryRoutes registers registry credential management endpoints.
func (ce *ControllerRouter) registerRegistryRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/registry/credentials", auth(ce.registryCredHandlers.HandleCredentials))
}

// registerHarborRoutes registers Harbor registry integration endpoints.
func (ce *ControllerRouter) registerHarborRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	mux.HandleFunc("/api/harbor/config", auth(ce.harborHandlers.HandleConfig))
	mux.HandleFunc("/api/harbor/projects", auth(ce.harborHandlers.ListProjects))
	mux.HandleFunc("/api/harbor/repositories", auth(ce.harborHandlers.ListRepositories))
	mux.HandleFunc("/api/harbor/repositories/{project}/{repo}/tags", auth(ce.harborHandlers.ListTags))
	mux.HandleFunc("/api/harbor/mappings", auth(ce.harborHandlers.HandleMappings))
	mux.HandleFunc("/api/harbor/mappings/{name}", auth(ce.harborHandlers.DeleteMapping))
	mux.HandleFunc("/api/harbor/sync", auth(ce.harborHandlers.Sync))
}

// registerAuthRoutes registers authentication and user management endpoints.
func (ce *ControllerRouter) registerAuthRoutes(mux *http.ServeMux, auth authMiddlewareFunc) {
	// Public — no auth required
	mux.HandleFunc("/api/auth/login", middlewares.LoggingMiddleware(ce.authHandlers.Login))

	// User authentication routes (accepts both user and agent tokens)
	mux.HandleFunc("/api/auth/logout", auth(ce.authHandlers.Logout))
	mux.HandleFunc("/api/auth/me", auth(ce.authHandlers.GetCurrentUser))
	mux.HandleFunc("/api/auth/change-password", auth(ce.authHandlers.ChangePassword))

	// User management routes (admin only — role checked in handlers)
	mux.HandleFunc("/api/users", auth(ce.authHandlers.HandleUsers))
	mux.HandleFunc("/api/users/{id}", auth(ce.authHandlers.HandleUserByID))
	mux.HandleFunc("/api/users/{id}/password", auth(ce.authHandlers.ResetPassword))
}
