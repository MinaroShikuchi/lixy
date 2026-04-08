package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/agent"
	"github.com/MinaroShikuchi/lixy/internal/daemon"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := daemon.Install(agentDaemonConfig()); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "uninstall":
			purge := len(os.Args) > 2 && os.Args[2] == "--purge"
			if err := daemon.Uninstall(agentDaemonConfig(), purge); err != nil {
				fmt.Fprintf(os.Stderr, "Uninstallation failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	app, err := agent.NewAgentApp(Version)
	if err != nil {
		panic(err)
	}
	app.Server.SetupHttpServer()
	app.Server.SetupSocketServer()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Server.StartSocketServer(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Server.StartHttpServer()
	}()

	// Start the reconciler
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Reconciler.Start(ctx)
	}()

	// Set up graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	app.Logger.Info("Shutting down server...")

	// Shutdown HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	app.Server.StopHttpServer(shutdownCtx)
	app.Server.StopSocketServer()

	// Signal all goroutines to stop
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()
	app.Logger.Info("Server shutdown complete")
}

func agentDaemonConfig() daemon.DaemonConfig {
	return daemon.DaemonConfig{
		ServiceName:  "lixies",
		DisplayName:  "Lixies Agent",
		Description:  "Lixies GitOps Agent for LXC Container Management",
		BinaryName:   "lixies",
		InstallDir:   "/opt/lixies",
		ConfigDir:    "/etc/lixy",
		DataDir:      "/var/lib/lixies",
		LogDir:       "/var/log/lixies",
		SocketDir:    "/run/lixies",
		User:         "lixies",
		Group:        "lixies",
		EnvVars:      map[string]string{},
		ConfigSource: "./lixies.yaml",
		ConfigDest:   "lixies.yaml",
	}
}
