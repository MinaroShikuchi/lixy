package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

type DeploymentReconciler struct {
	logger        *slog.Logger
	checkInterval time.Duration
	tokenService  *services.TokenService
	stopCh        chan struct{}
}

func NewDeploymentReconciler(logger *slog.Logger, checkInterval time.Duration, tokenService *services.TokenService) *DeploymentReconciler {
	return &DeploymentReconciler{
		logger:        logger,
		checkInterval: checkInterval,
		tokenService:  tokenService,
		stopCh:        make(chan struct{}),
	}
}

func (reconciler *DeploymentReconciler) Start(ctx context.Context) {
	reconciler.logger.Info("Starting deployment reconciler...")
	ticker := time.NewTicker(reconciler.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := reconciler.reconcile(); err != nil {
				reconciler.logger.Error("Failed to reconcile deployments", "error", err)
			}
		case <-reconciler.stopCh:
			ticker.Stop()
			return
		case <-ctx.Done():
			reconciler.logger.Info("Stopping deployment reconciler...")
			return
		}
	}
}

func (reconciler *DeploymentReconciler) Stop() {
	close(reconciler.stopCh)
}

func (reconciler *DeploymentReconciler) reconcile() error {
	reconciler.logger.Info("Reconciling deployments...")

	tokenData, err := reconciler.tokenService.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %v", err)
	}

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/api/deployments", tokenData.ControllerURL),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenData.Token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get deployments: %v", err)
	}
	reconciler.logger.Info("Successfully reconciled deployments")
	reconciler.logger.Info("Reconciliation complete")

	return nil
}

func (r *DeploymentReconciler) reconcileDeployment(desired store.DeploymentInfo, current store.DeploymentInfo) error {
	desiredMap := make(map[string]store.DeploymentInfo)
	currentMap := make(map[string]store.DeploymentInfo)

	for _, d := range []store.DeploymentInfo{desired} {
		desiredMap[d.Name] = d
	}
	for _, c := range []store.DeploymentInfo{current} {
		currentMap[c.Name] = c
	}

	// for name, desiredDeployment := range desiredMap {
	// 	if currentDeployment, exists := currentMap[name]; !exists {
	// 		// Deployment does not exist, create it
	// 		r.logger.Info("Creating deployment", "name", name)
	// 		err := r.deploymentService.CreateDeployment(
	// 			desiredDeployment.Name,
	// 			desiredDeployment.TargetLXC,
	// 			desiredDeployment.ComposeYML,
	// 		)
	return nil
}
