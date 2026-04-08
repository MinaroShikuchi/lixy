package controller

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/server"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

// ControllerApp is the composition root for the controller
type ControllerApp struct {
	Server        *server.Server
	HealthChecker *HealthChecker
	Logger        *slog.Logger
	Name          string
	Version       string
}

// NewControllerApp creates and wires up the controller application
func NewControllerApp(version string) (*ControllerApp, error) {
	// Load configuration
	cfg, err := LoadControllerConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Set up structured logging
	logger := middlewares.SetupLogger(cfg.LogLevel)

	// Log controller startup with version and configuration details
	logger.Info(cfg.Name, "version", version, "logLevel", cfg.LogLevel)

	// Initialize database connection
	db, err := store.InitDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize config store with encryption key from environment
	encryptionKey := os.Getenv("LIXY_ENCRYPTION_KEY")
	if encryptionKey == "" {
		// Fall back to JWT secret as encryption key
		encryptionKey = os.Getenv("LIXY_JWT_SECRET")
	}
	if encryptionKey == "" {
		logger.Warn("No encryption key provided via LIXY_ENCRYPTION_KEY or LIXY_JWT_SECRET. Data will not be encrypted.")
	}
	configStore, err := store.NewConfigStore(db, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config store: %w", err)
	}

	// Initialize agent store
	agentStore, err := store.NewAgentStore(db)
	if err != nil {
		logger.Error("Failed to initialize agent store", "error", err)
	}
	deploymentStore, err := store.NewDeploymentStore(db)
	if err != nil {
		logger.Error("Failed to initialize deployment store", "error", err)
	}

	// Initialize registry credential store
	registryCredStore, err := store.NewRegistryCredentialStore(db, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize registry credential store: %w", err)
	}

	// Initialize user store
	userStore, err := store.NewUserStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user store: %w", err)
	}

	// Get JWT secret for pull token service, user service, and auth service
	jwtSecret := os.Getenv("LIXY_JWT_SECRET")
	if jwtSecret == "" {
		logger.Warn("LIXY_JWT_SECRET not set, authentication services may not work correctly")
	}

	// Initialize auth service (replaces InitializeAuth)
	authService := services.NewAuthService(jwtSecret)

	// Initialize services
	agentService := services.NewAgentService(agentStore)
	deploymentService := services.NewDeploymentService(agentStore, deploymentStore)
	userService := services.NewUserService(userStore, jwtSecret)

	// Initialize default admin user if no admin users exist
	if err := userService.InitializeDefaultAdmin(); err != nil {
		logger.Error("Failed to initialize default admin user", "error", err)
	}

	// Git service now uses ConfigStore for persistence
	gitService := services.NewGitService(logger, configStore)
	registryService := services.NewRegistryService(logger, registryCredStore)
	parserService := services.NewGitOpsParser(logger)
	gitopsReconcilerService := services.NewGitOpsReconciler(logger, gitService, deploymentService, registryService, parserService, agentService)

	// Initialize GitHub App service (kept for potential GitHub API operations)
	// Note: Registry authentication now uses stored credentials exclusively
	_ = services.NewGitHubAppService(logger, configStore)

	// Initialize pull token service with registry credential store
	pullTokenService := services.NewPullTokenService(logger, registryCredStore, []byte(jwtSecret))

	// Create a closure for GetSystemInfo that captures version and port
	getSystemInfo := func() (*domain.SystemInfo, error) {
		return server.GetSystemInfo(version, cfg.Port)
	}

	// Initialize command handler
	commandHandler := NewControllerCommandHandler(logger, agentService, deploymentService, gitopsReconcilerService, registryService, authService, getSystemInfo)

	// Initialize endpoint handlers
	agentHandlers := handlers.NewAgentHandlers(agentService, authService, logger)
	deploymentHandlers := handlers.NewDeploymentHandlers(deploymentService, logger)
	healthCheckHandler := handlers.NewHealthCheckHandler(version)
	logHandlers := handlers.NewLogHandlers(cfg.LogFile)
	githubHandlers := handlers.NewGitHubHandlers(logger, configStore)
	gitopsHandlers := handlers.NewGitOpsHandlers(logger, gitService, gitopsReconcilerService)
	dashboardHandlers := handlers.NewDashboardHandlers(logger, agentService, deploymentService, gitService)
	pullTokenHandlers := handlers.NewPullTokenHandlers(logger, pullTokenService)
	registryCredHandlers := handlers.NewRegistryCredentialHandlers(logger, registryCredStore)
	authHandlers := handlers.NewAuthHandler(userService)

	// Initialize router
	endpointHandler := NewControllerRouter(RouterDeps{
		AgentHandlers:        agentHandlers,
		DeploymentHandlers:   deploymentHandlers,
		HealthCheckHandler:   healthCheckHandler,
		LogHandlers:          logHandlers,
		GitHubHandlers:       githubHandlers,
		DashboardHandlers:    dashboardHandlers,
		GitOpsHandlers:       gitopsHandlers,
		PullTokenHandlers:    pullTokenHandlers,
		RegistryCredHandlers: registryCredHandlers,
		AuthHandlers:         authHandlers,
		AuthService:          authService,
		UserService:          userService,
	})

	// Create server
	srv := server.NewServer(cfg.Port, cfg.SocketPath, logger, endpointHandler, commandHandler)

	// Parse the health check interval from configuration (string) to time.Duration
	checkInterval, err := time.ParseDuration(cfg.Health.CheckInterval)
	if err != nil {
		logger.Error("Invalid health check interval, using default 30s", "error", err)
		checkInterval = 30 * time.Second
	}
	healthChecker := NewHealthChecker(checkInterval, agentStore, logger)

	return &ControllerApp{
		Server:        srv,
		HealthChecker: healthChecker,
		Logger:        logger,
		Name:          cfg.Name,
		Version:       version,
	}, nil
}
