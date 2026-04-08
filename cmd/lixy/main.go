// cmd/lixy/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/controller"
	"github.com/MinaroShikuchi/lixy/internal/daemon"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := daemon.Install(controllerDaemonConfig()); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "uninstall":
			purge := len(os.Args) > 2 && os.Args[2] == "--purge"
			if err := daemon.Uninstall(controllerDaemonConfig(), purge); err != nil {
				fmt.Fprintf(os.Stderr, "Uninstallation failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	app, err := controller.NewControllerApp(Version)
	if err != nil {
		panic(err)
	}
	app.Server.SetupHttpServer()
	app.Server.SetupSocketServer()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

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

	app.HealthChecker.Start()
	defer app.HealthChecker.Stop()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	app.Logger.Info("Shutting down server...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	app.Server.StopHttpServer(shutdownCtx)
	app.Server.StopSocketServer()

	wg.Wait()
	app.Logger.Info("Server shutdown complete")
}

func controllerDaemonConfig() daemon.DaemonConfig {
	return daemon.DaemonConfig{
		ServiceName:  "lixy",
		DisplayName:  "Lixy Controller",
		Description:  "Lixy GitOps Controller for LXC Container Management",
		BinaryName:   "lixy",
		InstallDir:   "/opt/lixy",
		ConfigDir:    "/etc/lixy",
		DataDir:      "/var/lib/lixy",
		LogDir:       "/var/log/lixy",
		SocketDir:    "/run/lixy",
		User:         "lixy",
		Group:        "lixy",
		EnvVars:      map[string]string{},
		ConfigSource: "./lixy.yaml",
		ConfigDest:   "lixy.yaml",
	}
}
