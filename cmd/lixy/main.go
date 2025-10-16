// cmd/agent/main.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/MinaroShikuchi/lixy/internal/auth"
	"github.com/MinaroShikuchi/lixy/internal/controller"
	"github.com/MinaroShikuchi/lixy/internal/handlers"
	"github.com/MinaroShikuchi/lixy/internal/middlewares"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

// Command represents a request from the CLI
type Command struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

// Response represents the agent's response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const socketPath = "/tmp/lixy.sock"

var agentStore *store.AgentStore

func SetAgentStore(store *store.AgentStore) {
	agentStore = store
}

func main() {
	// Initialize authentication system
	if err := auth.InitializeAuth(); err != nil {
		log.Fatalf("Failed to initialize authentication: %v", err)
	}

	// Initialize agent store
	agentStore, err := store.NewAgentStore()
	if err != nil {
		log.Fatalf("Failed to initialize agent store: %v", err)
	}

	SetAgentStore(agentStore)
	// Set the agent store for the auth package
	handlers.SetAgentStore(agentStore)
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

	// Setup HTTP server for agent registration
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: setupHTTPHandlers(),
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
			go handleConnection(conn)
		}
	}()

	// Start HTTP server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("HTTP server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Create health checker with 5-minute interval
	healthChecker := controller.NewHealthChecker(agentStore, 5*time.Minute)

	// Start the health checker
	healthChecker.Start()
	defer healthChecker.Stop()

	// Set up graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	log.Println("Shutting down servers...")

	// Shutdown HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop accepting new socket connections
	socketListener.Close()

	// Signal the socket server to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("All servers have shut down")
}

func setupHTTPHandlers() http.Handler {
	mux := http.NewServeMux()
	// Create authenticated routes
	authenticatedAPI := http.NewServeMux()
	// authenticatedAPI.HandleFunc("/api/status", statusHandler)
	// Add authentication middleware for protected routes
	mux.Handle("/api/", middlewares.AuthenticateAgent(authenticatedAPI))

	// Add agent registration endpoints
	mux.HandleFunc("/api/register-agent", handlers.RegisterAgentHandler)
	mux.HandleFunc("/api/tokens/registration", handlers.GenerateRegistrationTokenHandler)
	// Admin routes for listing agents
	mux.HandleFunc("/admin/agents", handlers.ListAgentsHandler)

	return mux
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd Command
	if err := decoder.Decode(&cmd); err != nil {
		encoder.Encode(Response{Success: false, Message: "Invalid command format"})
		return
	}

	var response Response

	switch cmd.Action {
	case "get-deployments":
		// Handle list all deployments
		deployments := []map[string]string{
			{"name": "app1", "targetLXC": "101", "status": "running"},
			{"name": "app2", "targetLXC": "102", "status": "stopped"},
		}
		response = Response{Success: true, Data: deployments}

	case "get-deployment":
		// Handle get specific deployment
		name := cmd.Params["name"]
		// In a real implementation, look up the deployment by name
		deployment := map[string]string{
			"name":      name,
			"targetLXC": "101",
			"status":    "running",
			"image":     "my-app:latest",
			"created":   "2023-01-01 12:00:00",
		}
		response = Response{Success: true, Data: deployment}
	case "get-targets":
		// Get agents using the agent store
		agents := agentStore.ListAgents()
		// Convert agents to targets format
		targets := make([]map[string]string, 0, len(agents))
		for _, agent := range agents {
			target := map[string]string{
				"id":     agent.ID,
				"name":   agent.Name,
				"status": agent.Status,
				"ip":     agent.IP,
			}

			// Add selected metadata fields if needed
			for key, value := range agent.Metadata {
				// Only include specific metadata fields you want in the response
				if key == "version" || key == "os" || key == "arch" {
					target[key] = value
				}
			}

			targets = append(targets, target)
		}
		response = Response{Success: true, Data: targets}
	// Implement other CRUD operations similarly
	case "i-dont-know-yet":
		if err := controller.UpdateDeployment("101", "/opt/deployments/app1"); err != nil {
			log.Printf("Error updating deployment: %v", err)
		}

	default:
		response = Response{Success: false, Message: "Unknown command"}
	}

	encoder.Encode(response)
}
