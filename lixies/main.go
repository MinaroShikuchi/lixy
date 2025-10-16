package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
)

// Configuration for the lixies agent
type Config struct {
	Port        string
	LogLevel    string
	ComposeRoot string
}

// Global logger

func main() {
	// Initialize configuration
	config := Config{
		Port:        "8765",
		LogLevel:    "info",
		ComposeRoot: "/opt/deployments", // Default root directory for deployments
	}

	// Set up structured logging
	logger := middlewares.SetupLogger(config.LogLevel)

	// Log agent startup with version and configuration details
	logger.Info("Lixies agent starting",
		"version", "0.1.0",
		"port", config.Port,
		"logLevel", config.LogLevel)

	// Set up HTTP router with middleware
	mux := http.NewServeMux()

	// Add handlers with logging middleware
	mux.HandleFunc("/healthz", middlewares.LoggingMiddleware(handlers.HealthCheckHandler))
	mux.HandleFunc("/deploy", middlewares.LoggingMiddleware(handlers.DeployHandler))
	mux.HandleFunc("/update", middlewares.LoggingMiddleware(handlers.UpdateDeploymentHandler))

	// Create server with timeouts
	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Lixies agent listening for requests", "address", "0.0.0.0:"+config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server shutdown complete")
}
