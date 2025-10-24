package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// Client handles communication with the GitOps agent
type Client struct {
	Name            string
	Version         string
	Port            int
	LogLevel        string
	ComposeRoot     string
	SocketPath      string
	HttpServer      *http.Server
	sockerListener  net.Listener
	Logger          *slog.Logger
	EndpointHandler domain.EndpointHandler
	CommandHandler  domain.CommandHandler
}

// GetSystemInfo returns information about the agent's environment
func (c *Client) GetSystemInfo() (*domain.SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &domain.SystemInfo{
		Version:  c.Version,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		Port:     c.Port,
	}, nil
}

func (c *Client) SetupHttpServer() {
	// Create the mux
	mux := http.NewServeMux()

	// Add a nil check to prevent panic
	if c.EndpointHandler != nil {
		c.EndpointHandler.RegisterRoutes(mux)
	} else {
		// Either provide default routes or log a warning
		log.Printf("Warning: EndpointHandler is nil, no routes registered")
	}

	// Let the appropriate endpoint handler register routes on this mux
	c.HttpServer = &http.Server{
		Addr:         ":" + fmt.Sprint(c.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
}

func (c *Client) StartHttpServer() {
	c.Logger.Info("Starting HTTP server", "address", c.HttpServer.Addr)
	if err := c.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		c.Logger.Error("Failed to start server", "error", err)
	}
}

func (c *Client) SetupSocketServer() {
	// Setup the Unix socket server (existing code)
	if err := os.RemoveAll(c.SocketPath); err != nil {
		c.Logger.Error("Failed to remove existing socket", "error", err)
	}

	if err := os.MkdirAll(filepath.Dir(c.SocketPath), 0755); err != nil {
		c.Logger.Error("Failed to create socket directory", "error", err)
	}

	socketListener, err := net.Listen("unix", c.SocketPath)
	if err != nil {
		c.Logger.Error("Failed to listen on socket", "error", err)
	}
	c.sockerListener = socketListener

	if err := os.Chmod(c.SocketPath, 0660); err != nil {
		c.Logger.Error("Failed to set socket permissions", "error", err)
	}
}

func (c *Client) StartSocketServer(ctx context.Context) {
	c.Logger.Info("Starting Unix socket server", "address", c.sockerListener.Addr().String())
	for {
		conn, err := c.sockerListener.Accept()
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
		go c.handleConnection(conn)
	}
}

func (c *Client) StopHttpServer(ctx context.Context) {
	// c.Logger.Info("Shutting down Lixies HTTP client")
	if err := c.HttpServer.Shutdown(ctx); err != nil {
		c.Logger.Error("Failed to shut down server gracefully", "error", err)
	}
}

func (c *Client) StopSocketServer() {
	// c.Logger.Info("Shutting down Lixies Unix socket server")
	if err := c.sockerListener.Close(); err != nil {
		c.Logger.Error("Failed to close socket listener", "error", err)
	}
}

// handleConnection processes a single Unix socket connection
func (c *Client) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd domain.Command
	if err := decoder.Decode(&cmd); err != nil {
		encoder.Encode(domain.Response{Success: false, Message: "Invalid command format"})
		return
	}
	// Use the command handler interface instead of conditionals
	response := c.CommandHandler.HandleCommand(cmd)
	encoder.Encode(response)
}
