package agent

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/server"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

// AgentApp is the composition root for the agent
type AgentApp struct {
	Server     *server.Server
	Reconciler *DeploymentReconciler
	Logger     *slog.Logger
	Name       string
	Version    string
}

// NewAgentApp creates and wires up the agent application
func NewAgentApp(version string) (*AgentApp, error) {
	cfg, err := LoadAgentConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Set up structured logging
	logger := middlewares.SetupLogger(cfg.LogLevel)

	// Log agent startup with version and configuration details
	logger.Info(cfg.Name, "version", version, "logLevel", cfg.LogLevel)

	// Detect and verify container runtime (Docker or Podman)
	rt, err := DetectContainerRuntime(logger)
	if err != nil {
		logger.Error("Failed to detect container runtime", "error", err)
		logger.Error("Container runtime check failed - agent will start but deployments may fail",
			"required", "Docker or Podman must be installed")
	} else {
		// Verify compose tool is available
		if err := VerifyDockerCompose(rt, logger); err != nil {
			logger.Warn("Compose tool not found", "error", err)
			logger.Warn("Deployments requiring docker-compose may fail",
				"suggestion", "Install docker-compose or podman-compose")
		}
	}

	// Initialize database connection
	db, err := store.InitDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize stores
	tokenStore, err := store.NewTokenStore(db)
	if err != nil {
		logger.Error("Failed to initialize token store", "error", err)
	}

	// Initialize services
	tokenService := services.NewTokenService(tokenStore)

	// Create a closure for GetSystemInfo that captures version and port
	getSystemInfo := func() (*domain.SystemInfo, error) {
		return server.GetSystemInfo(version, cfg.Port)
	}

	registrationService := services.NewAgentRegistrationService(logger, tokenService, getSystemInfo)

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
	commandHandler := NewAgentCommandHandler(logger, registrationService)

	// Initialize router
	endpointHandler := NewAgentRouter()

	// Create server
	srv := server.NewServer(cfg.Port, cfg.SocketPath, logger, endpointHandler, commandHandler)

	// Initialize deployment runner with detected runtime and pull token client
	deploymentRunner := NewDeploymentRunner(logger, rt, pullTokenClient)

	// Parse the reconciler check interval from configuration (string) to time.Duration
	checkInterval, err := time.ParseDuration(cfg.Reconciler.CheckInterval)
	if err != nil {
		logger.Error("Invalid reconciler check interval, using default 1m", "error", err)
		checkInterval = time.Minute
	}
	reconciler := NewDeploymentReconciler(logger, checkInterval, tokenService, deploymentRunner)

	return &AgentApp{
		Server:     srv,
		Reconciler: reconciler,
		Logger:     logger,
		Name:       cfg.Name,
		Version:    version,
	}, nil
}
