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
	c := client.NewControllerClient(Version, 8080, "info")
	c.SetupHttpServer()
	c.SetupSocketServer()

	// Create a WaitGroup for coordinating shutdown
	var wg sync.WaitGroup

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Start Unix socket server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.StartSocketServer(ctx)
	}()

	// Start HTTP server in a goroutine
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

	// IMPORTANT: First cancel the context to signal all operations to stop
	cancel()

	// Then initiate server shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop HTTP and socket servers
	c.StopHttpServer(shutdownCtx)
	c.StopSocketServer()

	// Wait for all goroutines to finish
	wg.Wait()
	c.Logger.Info("Server shutdown complete")
}
