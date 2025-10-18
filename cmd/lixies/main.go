package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/handlers"
	client "github.com/MinaroShikuchi/lixy/internal/lixies-client"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/MinaroShikuchi/lixy/pkg/types"
)

// Configuration for the lixies agent
type Config struct {
	Port        string
	LogLevel    string
	ComposeRoot string
}

const socketPath = "/tmp/lixies.sock"

var tokenStore *store.TokenStore

func setTokenStore(store *store.TokenStore) {
	tokenStore = store
}

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

	// Initialize token store
	tokenStore, err := store.NewTokenStore()
	if err != nil {
		logger.Error("Failed to initialize token store", "error", err)
	}
	setTokenStore(tokenStore)

	// Create a WaitGroup for coordinating shutdown
	var wg sync.WaitGroup

	// Setup the Unix socket server (existing code)
	if err := os.RemoveAll(socketPath); err != nil {
		log.Fatalf("Failed to remove existing socket: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		log.Fatalf("Failed to create socket directory: %v", err)
	}

	socketListener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on socket: %v", err)
	}

	if err := os.Chmod(socketPath, 0660); err != nil {
		log.Fatalf("Failed to set socket permissions: %v", err)
	}

	// Set up HTTP router with middleware
	mux := http.NewServeMux()

	// Add handlers with logging middleware
	mux.HandleFunc("/healthz", middlewares.LoggingMiddleware(handlers.HealthCheckHandler))
	mux.HandleFunc("/deploy", middlewares.LoggingMiddleware(handlers.DeployHandler))
	mux.HandleFunc("/update", middlewares.LoggingMiddleware(handlers.UpdateDeploymentHandler))

	// Create server with timeouts
	httpServer := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Unix socket server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Unix socket server listening on %s", socketPath)

		for {
			conn, err := socketListener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					// Server is shutting down
					return
				default:
					log.Printf("Failed to accept socket connection: %v", err)
					continue
				}
			}

			// Handle each connection in a separate goroutine
			go handleConnection(conn, logger)
		}
	}()

	// Start server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Lixies agent listening for requests", "address", ":"+config.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
		}
	}()

	// Set up graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	logger.Info("Shutting down server...")

	// Shutdown HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	// Stop accepting new socket connections
	socketListener.Close()

	// Signal the socket server to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	logger.Info("Server shutdown complete")
}

// handleConnection processes a single Unix socket connection
func handleConnection(conn net.Conn, logger *slog.Logger) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd types.Command
	if err := decoder.Decode(&cmd); err != nil {
		encoder.Encode(types.Response{Success: false, Message: "Invalid command format"})
		return
	}

	var response types.Response

	switch cmd.Action {
	case "register-agent":
		// client.RegisterWithController(*tokenStore, cmd.Params["controller"], cmd.Params["token"], cmd.Params["name"])
		if err := client.RegisterWithController(*tokenStore, cmd.Params["controller"], cmd.Params["token"], cmd.Params["name"]); err != nil {
			logger.Error("Agent registration failed", "error", err)
			response = types.Response{Success: false, Message: "Registration failed: " + err.Error()}
			break
		}
		response = types.Response{Success: true, Message: "Agent registered successfully"}
	default:
		response = types.Response{Success: false, Message: "Unknown command"}
	}

	encoder.Encode(response)
}
