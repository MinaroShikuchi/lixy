package client

import (
	"fmt"
	"os"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/agent"
	"github.com/MinaroShikuchi/lixy/internal/controller"
	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

func NewControllerClient(version string) *Client {
	// Load configuration
	cfg, err := LoadControllerConfig()
	if err != nil {
		fmt.Printf("failed to load config: %v", err)
		return nil
	}

	// Implementation
	client := &Client{
		Name:       cfg.Name,
		Version:    version,
		Port:       cfg.Port,
		LogLevel:   cfg.LogLevel,
		SocketPath: cfg.SocketPath,
	}
	// Set up structured logging
	logger := middlewares.SetupLogger(cfg.LogLevel)
	client.Logger = logger

	// Log agent startup with version and configuration details
	logger.Info(client.Name, "version", version, "logLevel", cfg.LogLevel)

	// Initialize authentication system
	if err := services.InitializeAuth(); err != nil {
		logger.Error("Failed to initialize authentication", "error", err)
	}

	// Initialize database connection
	db, err := store.InitDB(cfg.Database)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		return nil
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
		logger.Error("Failed to initialize config store", "error", err)
		return nil
	}

	// Initialize token store
	// tokenStore, err := store.NewTokenStore()
	// if err != nil {
	// 	logger.Error("Failed to initialize token store", "error", err)
	// }

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
		logger.Error("Failed to initialize registry credential store", "error", err)
		return nil
	}

	// Initialize user store
	userStore, err := store.NewUserStore(db)
	if err != nil {
		logger.Error("Failed to initialize user store", "error", err)
		return nil
	}

	// Get JWT secret for pull token service and user service
	jwtSecret := os.Getenv("LIXY_JWT_SECRET")
	if jwtSecret == "" {
		logger.Warn("LIXY_JWT_SECRET not set, authentication services may not work correctly")
	}

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

	// Initialize command handler
	client.CommandHandler = controller.NewControllerCommandHandler(client.Logger, agentService, deploymentService, gitopsReconcilerService, registryService, client.GetSystemInfo)

	// Initialize endpoint handlers
	agentHandlers := handlers.NewAgentHandlers(agentService, logger)
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
	client.EndpointHandler = controller.NewControllerRouter(agentHandlers, deploymentHandlers, healthCheckHandler, logHandlers, githubHandlers, dashboardHandlers, gitopsHandlers, pullTokenHandlers, registryCredHandlers, authHandlers, userService)
	// parse the health check interval from configuration (string) to time.Duration
	checkInterval, err := time.ParseDuration(cfg.Health.CheckInterval)
	if err != nil {
		logger.Error("Invalid health check interval, using default 30s", "error", err)
		checkInterval = 30 * time.Second
	}
	client.HealthChecker = NewHealthChecker(checkInterval, agentStore, logger)

	return client

}

func NewAgentClient(version string) *Client {
	cfg, err := LoadAgentConfig()
	if err != nil {
		fmt.Printf("failed to load config: %v", err)
		panic(err)
	}
	// Implementation
	client := &Client{
		Name:       cfg.Name,
		Version:    version,
		Port:       cfg.Port,
		LogLevel:   cfg.LogLevel,
		SocketPath: cfg.SocketPath,
	}

	// Set up structured logging
	logger := middlewares.SetupLogger(cfg.LogLevel)
	client.Logger = logger
	// Log agent startup with version and configuration details
	logger.Info(client.Name, "version", version, "logLevel", cfg.LogLevel)

	// Detect and verify container runtime (Docker or Podman)
	runtime, err := DetectContainerRuntime(logger)
	if err != nil {
		logger.Error("Failed to detect container runtime", "error", err)
		logger.Error("Container runtime check failed - agent will start but deployments may fail",
			"required", "Docker or Podman must be installed")
	} else {
		// Verify compose tool is available
		if err := VerifyDockerCompose(runtime, logger); err != nil {
			logger.Warn("Compose tool not found", "error", err)
			logger.Warn("Deployments requiring docker-compose may fail",
				"suggestion", "Install docker-compose or podman-compose")
		}
	}

	// Initialize database connection
	db, err := store.InitDB(cfg.Database)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		panic(err)
	}
	// Initialize stores
	tokenStore, err := store.NewTokenStore(db)
	if err != nil {
		logger.Error("Failed to initialize token store", "error", err)
	}
	// Initialize services
	tokenService := services.NewTokenService(tokenStore)
	registrationService := services.NewAgentRegistrationService(logger, tokenService, client.GetSystemInfo)

	// Get controller URL and agent name for pull token client
	tokenData, err := tokenService.GetToken()
	var pullTokenClient *PullTokenClient
	if err == nil && tokenData.ControllerURL != "" {
		// Initialize pull token client with token service for authentication
		pullTokenClient = NewPullTokenClient(logger, tokenData.ControllerURL, cfg.Name, tokenService)
		logger.Info("Initialized pull token client", "controller_url", tokenData.ControllerURL)
	} else {
		logger.Warn("No controller URL available, pull token client not initialized")
	}

	// Initialize command handler with registration service
	client.CommandHandler = agent.NewAgentCommandHandler(client.Logger, registrationService)
	// Initialize router
	client.EndpointHandler = agent.NewAgentRouter()

	// Initialize deployment runner with detected runtime and pull token client
	deploymentRunner := NewDeploymentRunner(logger, runtime, pullTokenClient)
	// parse the reconciler check interval from configuration (string) to time.Duration
	checkInterval, err := time.ParseDuration(cfg.Reconciler.CheckInterval)
	if err != nil {
		logger.Error("Invalid reconciler check interval, using default 1m", "error", err)
		checkInterval = time.Minute
	}
	client.Reconciler = NewDeploymentReconciler(logger, checkInterval, tokenService, deploymentRunner)
	return client

}
