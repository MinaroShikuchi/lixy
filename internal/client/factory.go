package client

import (
	"database/sql"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/agent"
	"github.com/MinaroShikuchi/lixy/internal/controller"
	"github.com/MinaroShikuchi/lixy/internal/controller/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

func NewControllerClient(version string, port int, logLevel string) *Client {
	// Implementation
	client := &Client{
		Name:       "Lixy Controller",
		Version:    version,
		Port:       port,
		LogLevel:   logLevel,
		SocketPath: "/tmp/lixy.sock",
	}
	// Set up structured logging
	logger := middlewares.SetupLogger(logLevel)
	client.Logger = logger

	// Log agent startup with version and configuration details
	logger.Info(client.Name, "version", version, "logLevel", logLevel)

	// Initialize authentication system
	if err := services.InitializeAuth(); err != nil {
		logger.Error("Failed to initialize authentication", "error", err)
	}

	db, err := sql.Open("sqlite3", "lixy.db")
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		return nil
	}
	// Enable foreign key enforcement for SQLite
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		logger.Error("Failed to enable foreign keys", "error", err)
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
	// Initialize router
	client.EndpointHandler = controller.NewControllerRouter(agentHandlers, deploymentHandlers)

	client.HealthChecker = NewHealthChecker(5*time.Minute, agentStore, logger)

	return client

}

func NewAgentClient(version string, port int, logLevel string) *Client {
	// Implementation
	client := &Client{
		Name:       "Lixy Agent",
		Version:    version,
		Port:       port,
		LogLevel:   logLevel,
		SocketPath: "/tmp/lixies.sock",
	}

	// Set up structured logging
	logger := middlewares.SetupLogger(logLevel)
	client.Logger = logger
	// Log agent startup with version and configuration details
	logger.Info(client.Name, "version", version, "logLevel", logLevel)

	db, err := sql.Open("sqlite3", "lixies.db")
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		return nil
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
	client.Reconciler = NewDeploymentReconciler(logger, 30*time.Second, tokenService, deploymentRunner)
	return client

}
