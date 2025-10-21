package client

import (
	"log"

	"github.com/MinaroShikuchi/lixy/internal/agent"
	"github.com/MinaroShikuchi/lixy/internal/auth"
	"github.com/MinaroShikuchi/lixy/internal/controller"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

func NewControllerClient(version string, port string, logLevel string) *Client {
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
	if err := auth.InitializeAuth(); err != nil {
		logger.Error("Failed to initialize authentication", "error", err)
	}

	// Initialize token store
	tokenStore, err := store.NewTokenStore()
	if err != nil {
		logger.Error("Failed to initialize token store", "error", err)
	}
	client.TokenStore = tokenStore

	// Initialize agent store
	agentStore, err := store.NewAgentStore()
	if err != nil {
		log.Fatalf("Failed to initialize agent store: %v", err)
	}
	client.AgentStore = agentStore
	client.EndpointHandler = controller.NewControllerRouter()
	client.CommandHandler = controller.NewControllerCommandHandler(client.Logger, client.AgentStore)
	return client

}

func NewAgentClient(version string, port string, logLevel string) *Client {
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
	client.EndpointHandler = agent.NewAgentRouter()
	client.CommandHandler = agent.NewAgentCommandHandler(client.Logger, client.TokenStore)
	return client

}
