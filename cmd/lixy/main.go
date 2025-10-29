// cmd/agent/main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	client "github.com/MinaroShikuchi/lixy/internal/client"
)

// Configuration for the lyxi controller
type Config struct {
	Port     string
	LogLevel string
}

func main() {

	// c := client.LixiesHttpClient("Lixy Controller", "0.1.0", 8080, "info", "/tmp/lixy.sock")
	c := client.NewControllerClient(Version, 8080, "info")
	c.SetupHttpServer()
	c.SetupSocketServer()

	// Create a WaitGroup for coordinating shutdown
	var wg sync.WaitGroup

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Unix socket server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.StartSocketServer(ctx)
	}()

	// Start server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.StartHttpServer()
	}()

	// Start the health checker
	c.HealthChecker.Start()
	defer c.HealthChecker.Stop()

	// Set up graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	c.Logger.Info("Shutting down server...")

	// Shutdown HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	c.StopHttpServer(shutdownCtx)
	c.StopSocketServer()

	// Signal the socket server to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	c.Logger.Info("Server shutdown complete")
}
