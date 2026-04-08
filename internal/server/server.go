package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/rs/cors"
)

// Server handles HTTP and Unix socket server lifecycle
type Server struct {
	Port            int
	SocketPath      string
	HttpServer      *http.Server
	socketListener  net.Listener
	Logger          *slog.Logger
	EndpointHandler domain.EndpointHandler
	CommandHandler  domain.CommandHandler
}

// NewServer creates a new server instance
func NewServer(port int, socketPath string, logger *slog.Logger, endpointHandler domain.EndpointHandler, commandHandler domain.CommandHandler) *Server {
	return &Server{
		Port:            port,
		SocketPath:      socketPath,
		Logger:          logger,
		EndpointHandler: endpointHandler,
		CommandHandler:  commandHandler,
	}
}

func (s *Server) SetupHttpServer() {
	mux := http.NewServeMux()
	s.EndpointHandler.RegisterRoutes(mux)
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})
	s.HttpServer = &http.Server{
		Addr:         ":" + fmt.Sprint(s.Port),
		Handler:      corsHandler.Handler(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
}

func (s *Server) StartHttpServer() {
	s.Logger.Info("Starting HTTP server", "address", s.HttpServer.Addr)
	if err := s.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.Logger.Error("Failed to start server", "error", err)
	}
}

func (s *Server) SetupSocketServer() {
	if err := os.RemoveAll(s.SocketPath); err != nil {
		s.Logger.Error("Failed to remove existing socket", "error", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.SocketPath), 0755); err != nil {
		s.Logger.Error("Failed to create socket directory", "error", err)
	}

	socketListener, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		s.Logger.Error("Failed to listen on socket", "error", err)
	}
	s.socketListener = socketListener

	if err := os.Chmod(s.SocketPath, 0660); err != nil {
		s.Logger.Error("Failed to set socket permissions", "error", err)
	}
}

func (s *Server) StartSocketServer(ctx context.Context) {
	s.Logger.Info("Starting Unix socket server", "socketPath", s.SocketPath)
	for {
		// Check context before attempting to accept
		select {
		case <-ctx.Done():
			s.Logger.Info("Socket server shutting down gracefully")
			return
		default:
			// Continue only if we're not shutting down
		}

		// Set a timeout to unblock Accept periodically
		if unixListener, ok := s.socketListener.(*net.UnixListener); ok {
			unixListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		// Accept a connection
		conn, err := s.socketListener.Accept()
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

			s.Logger.Error("Failed to accept connection", "error", err)
			continue
		}

		// Handle the connection
		go s.handleConnection(conn)
	}
}

func (s *Server) StopHttpServer(ctx context.Context) {
	if err := s.HttpServer.Shutdown(ctx); err != nil {
		s.Logger.Error("Failed to shut down server gracefully", "error", err)
	}
}

func (s *Server) StopSocketServer() {
	if s.socketListener != nil {
		s.Logger.Info("Closing socket listener")
		s.socketListener.Close()

		// Clean up socket file if needed
		if unixListener, ok := s.socketListener.(*net.UnixListener); ok {
			addr := unixListener.Addr().(*net.UnixAddr)
			os.Remove(addr.Name)
		}
	}
}

// handleConnection processes a single Unix socket connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var cmd domain.Command
	if err := decoder.Decode(&cmd); err != nil {
		encoder.Encode(domain.Response{Success: false, Message: "Invalid command format"})
		return
	}
	// Use the command handler interface instead of conditionals
	response := s.CommandHandler.HandleCommand(cmd)
	encoder.Encode(response)
}
