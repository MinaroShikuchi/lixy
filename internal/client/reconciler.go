package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

type DeploymentReconciler struct {
	logger        *slog.Logger
	checkInterval time.Duration
	tokenService  *services.TokenService
	runner        *DeploymentRunner
	stopCh        chan struct{}
}

func NewDeploymentReconciler(logger *slog.Logger, checkInterval time.Duration, tokenService *services.TokenService, runner *DeploymentRunner) *DeploymentReconciler {
	// create a deployments dir if it doesn't exist
	if err := os.MkdirAll("./deployments", 0755); err != nil {
		logger.Error("Failed to create deployments directory", "error", err)
	}

	return &DeploymentReconciler{
		logger:        logger,
		checkInterval: checkInterval,
		tokenService:  tokenService,
		runner:        runner,
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
			tokenData, err := reconciler.tokenService.GetToken()
			if err != nil {
				reconciler.logger.Error("Failed to get token; skipping reconciliation", "error", err)
				continue
			}
			if tokenData == (store.TokenData{}) {
				reconciler.logger.Info("No token found; skipping reconciliation")
				continue
			}
			if err := reconciler.reconcile(tokenData); err != nil {
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

func (reconciler *DeploymentReconciler) reconcile(tokenData store.TokenData) error {
	reconciler.logger.Info("Reconciling deployments...")

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

	deployments := make([]domain.DeploymentDto, 0)
	if err := json.NewDecoder(resp.Body).Decode(&deployments); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	reconciler.logger.Info("Fetched desired deployments", "count", len(deployments))

	currentDeployments, err := reconciler.runner.ListRunning()
	if err != nil {
		return fmt.Errorf("failed to list running deployments: %v", err)
	}

	reconciler.logger.Info("Fetched current deployments", "count", len(currentDeployments))

	reconciler.reconcileDeployment(deployments, currentDeployments)

	reconciler.logger.Info("Successfully reconciled deployments")

	return nil
}

func (r *DeploymentReconciler) reconcileDeployment(desired []domain.DeploymentDto, current []store.DeploymentInfo) error {
	desiredByName := make(map[string]store.DeploymentInfo)
	currentByName := make(map[string]store.DeploymentInfo)

	for _, d := range desired {
		desiredByName[d.Name] = store.DeploymentInfo{
			ID:          d.ID,
			Name:        d.Name,
			TargetLXC:   d.TargetLXC,
			ComposeYAML: d.ComposeYAML,
			Status:      d.Status,
		}
	}
	for _, d := range current {
		currentByName[d.Name] = d
	}
	// Find deployments to delete
	for name := range currentByName {
		if _, exists := desiredByName[name]; !exists {
			// Deployment should not exist, delete it
			r.logger.Info("Deleting deployment", "name", name)
			if err := r.runner.Delete(name); err != nil {
				r.logger.Error("Failed to delete deployment", "name", name, "error", err)
				continue
			}
			// r.reportStatus(deployment.ID, "Deleted", "")
			r.logger.Info("Deleted deployment", "name", name)
		}
	}

	// Find deployments to create or update (by name)
	for name, deployment := range desiredByName {
		if _, exists := currentByName[name]; !exists {
			// Deployment doesn't exist, create it
			r.logger.Info("Creating deployment", "name", name, "id", deployment.ID)
			if err := r.runner.Create(deployment.Name, deployment.ComposeYAML); err != nil {
				r.logger.Error("Failed to create deployment", "name", name, "id", deployment.ID, "error", err)
				continue
			}
			r.reportStatus(name, "running")
			r.logger.Info("Created deployment", "name", name, "id", deployment.ID)
		} else {
			// Deployment with same name already exists; could compare ComposeYAML/version here to decide updates
			r.logger.Debug("Deployment already exists", "name", name, "id", currentByName[name].ID)
			// compare the ComposeYAML to see if an update is needed
			if string(deployment.ComposeYAML) != string(currentByName[name].ComposeYAML) {
				r.logger.Info("Updating deployment", "name", name, "id", deployment.ID)
				if err := r.runner.Update(domain.DeploymentRequest{
					Name:        deployment.Name,
					ComposeYAML: deployment.ComposeYAML,
				},
				); err != nil {
					r.reportStatus(name, "failed")

					r.logger.Error("Failed to update deployment", "name", name, "id", deployment.ID, "error", err)
					continue
				}
				r.reportStatus(name, "running")
				r.logger.Info("Updated deployment", "name", name, "id", deployment.ID)
			} else {
				r.logger.Info("No update needed for deployment", "name", name, "id", deployment.ID)
			}
		}
	}

	return nil
}

func (r *DeploymentReconciler) reportStatus(name, status string) {
	tokenData, err := r.tokenService.GetToken()
	if err != nil {
		r.logger.Error("Failed to get token for status report", "name", name, "error", err)
	}

	reqBody := domain.DeploymentStatusUpdate{
		Status: status,
	}
	bodyBytes, err := json.Marshal(reqBody)

	if err != nil {
		r.logger.Error("Failed to marshal status update", "name", name, "error", err)
		return
	}

	req, err := http.NewRequest(
		"PATCH",
		fmt.Sprintf("%s/api/deployments/%s", tokenData.ControllerURL, name),
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		r.logger.Error("Failed to create status update request", "name", name, "error", err)
		return
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenData.Token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		r.logger.Error("Failed to send status update request", "name", name, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Failed to update deployment status", "name", name, "status_code", resp.StatusCode)
		return
	}

	r.logger.Info("Successfully updated deployment status", "name", name, "status", status)
}
