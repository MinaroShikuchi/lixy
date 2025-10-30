package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// Controller client  handles communication with the agent
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
	HealthChecker   *HealthChecker
	Reconciler      *DeploymentReconciler
}

// GetSystemInfo returns information about the agent's environment
func (c *Client) GetSystemInfo() (*domain.SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// get the ip addresss of the machine
	ip := ""

	// Iterate network interfaces and pick the first non-loopback IPv4 address
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			var ipnet *net.IPNet
			switch v := addr.(type) {
			case *net.IPNet:
				ipnet = v
			case *net.IPAddr:
				ipnet = &net.IPNet{IP: v.IP, Mask: v.IP.DefaultMask()}
			}
			if ipnet == nil {
				continue
			}
			ipAddr := ipnet.IP
			if ipAddr == nil || ipAddr.IsLoopback() {
				continue
			}
			if ip4 := ipAddr.To4(); ip4 != nil {
				ip = ip4.String()
				break
			}
		}
	}

	// Return SystemInfo with discovered IP
	return &domain.SystemInfo{
		Version:  c.Version,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		IP:       ip,
		Port:     c.Port,
	}, nil
}

func (c *Client) SetupHttpServer() {
	mux := http.NewServeMux()
	c.EndpointHandler.RegisterRoutes(mux)
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
	c.Logger.Info("Starting Unix socket server", "socketPath", c.SocketPath)
	for {
		// Check context before attempting to accept
		select {
		case <-ctx.Done():
			c.Logger.Info("Socket server shutting down gracefully")
			return
		default:
			// Continue only if we're not shutting down
		}

		// Set a timeout to unblock Accept periodically
		if unixListener, ok := c.sockerListener.(*net.UnixListener); ok {
			unixListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		// Accept a connection
		conn, err := c.sockerListener.Accept()
		if err != nil {
			// Handle specific error types differently
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				// This is just our periodic timeout, continue silently
				continue
			}

			if ctx.Err() != nil {
				// We're shutting down, exit gracefully
				return
			}

			if strings.Contains(err.Error(), "use of closed network connection") {
				// Socket was closed, likely during shutdown
				return
			}

			c.Logger.Error("Failed to accept connection", "error", err)
			continue
		}

		// Handle the connection
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
	if c.sockerListener != nil {
		c.Logger.Info("Closing socket listener")
		c.sockerListener.Close()

		// Clean up socket file if needed
		if unixListener, ok := c.sockerListener.(*net.UnixListener); ok {
			addr := unixListener.Addr().(*net.UnixAddr)
			os.Remove(addr.Name)
		}
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
