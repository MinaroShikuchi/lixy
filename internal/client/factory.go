package client

import (
	"fmt"
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
		fmt.Printf("failed to load config: %w", err)
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
	// Initialize services
	agentService := services.NewAgentService(agentStore)
	deploymentService := services.NewDeploymentService(agentStore, deploymentStore)
	// Initialize command handler
	client.CommandHandler = controller.NewControllerCommandHandler(client.Logger, agentService, deploymentService, client.GetSystemInfo)
	// Initialize endpoint handlers
	agentHandlers := handlers.NewAgentHandlers(agentService, logger)
	deploymentHandlers := handlers.NewDeploymentHandlers(deploymentService, logger)
	healthCheckHandler := handlers.NewHealthCheckHandler(version)
	logHandlers := handlers.NewLogHandlers(cfg.LogFile)
	// Initialize router
	client.EndpointHandler = controller.NewControllerRouter(agentHandlers, deploymentHandlers, healthCheckHandler, logHandlers)

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
		fmt.Printf("failed to load config: %w", err)
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
	// Initialize command handler
	client.CommandHandler = agent.NewAgentCommandHandler(client.Logger, tokenService, client.GetSystemInfo)
	// Initialize router
	client.EndpointHandler = agent.NewAgentRouter()

	// Initialize deployment runner
	deploymentRunner := NewDeploymentRunner(logger)
	// parse the reconciler check interval from configuration (string) to time.Duration
	checkInterval, err := time.ParseDuration(cfg.Reconciler.CheckInterval)
	if err != nil {
		logger.Error("Invalid reconciler check interval, using default 1m", "error", err)
		checkInterval = time.Minute
	}
	client.Reconciler = NewDeploymentReconciler(logger, checkInterval, tokenService, deploymentRunner)
	return client

}
