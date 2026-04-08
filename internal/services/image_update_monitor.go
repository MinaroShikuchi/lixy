package services

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
	"gopkg.in/yaml.v3"
)

// ImageUpdateMonitor monitors container registries for image updates
// and automatically reconciles deployments based on configured strategies
type ImageUpdateMonitor struct {
	logger          *slog.Logger
	registryService *RegistryService
	reconciler      *GitOpsReconciler
	gitService      *GitService
	updateHistory   *store.UpdateHistoryStore
	monitorState    *store.MonitorStateStore
	parser          *GitOpsParser

	// Configuration
	defaultInterval time.Duration
	enabled         bool

	// Runtime state
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  bool
}

// NewImageUpdateMonitor creates a new image update monitor
func NewImageUpdateMonitor(
	logger *slog.Logger,
	registryService *RegistryService,
	reconciler *GitOpsReconciler,
	gitService *GitService,
	updateHistory *store.UpdateHistoryStore,
	monitorState *store.MonitorStateStore,
	parser *GitOpsParser,
	defaultInterval time.Duration,
) *ImageUpdateMonitor {
	return &ImageUpdateMonitor{
		logger:          logger,
		registryService: registryService,
		reconciler:      reconciler,
		gitService:      gitService,
		updateHistory:   updateHistory,
		monitorState:    monitorState,
		parser:          parser,
		defaultInterval: defaultInterval,
		enabled:         false, // Disabled by default
		running:         false,
	}
}

// Start starts the background monitoring loop
func (m *ImageUpdateMonitor) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("monitor already running")
	}

	if !m.enabled {
		m.logger.Info("Image update monitor is disabled, not starting")
		return nil
	}

	m.running = true
	m.stopChan = make(chan struct{})

	m.wg.Add(1)
	go m.monitorLoop()

	m.logger.Info("Image update monitor started",
		"interval", m.defaultInterval)
	return nil
}

// Stop gracefully stops the monitoring loop
func (m *ImageUpdateMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("Stopping image update monitor...")
	close(m.stopChan)
	m.running = false

	// Wait for monitor loop to finish
	m.wg.Wait()

	m.logger.Info("Image update monitor stopped")
	return nil
}

// Enable enables the monitor
func (m *ImageUpdateMonitor) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
	m.logger.Info("Image update monitor enabled")
}

// Disable disables the monitor
func (m *ImageUpdateMonitor) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
	m.logger.Info("Image update monitor disabled")
}

// IsRunning returns whether the monitor is currently running
func (m *ImageUpdateMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IsEnabled returns whether the monitor is enabled
func (m *ImageUpdateMonitor) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// monitorLoop is the main background loop that checks for updates
func (m *ImageUpdateMonitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.defaultInterval)
	defer ticker.Stop()

	// Run initial check immediately
	m.checkAllEnvironments()

	for {
		select {
		case <-ticker.C:
			m.checkAllEnvironments()
		case <-m.stopChan:
			m.logger.Info("Monitor loop stopped")
			return
		}
	}
}

// checkAllEnvironments checks all environments for updates
func (m *ImageUpdateMonitor) checkAllEnvironments() {
	m.logger.Debug("Starting update check cycle")

	// Get all active monitor states
	states, err := m.monitorState.GetActiveDeployments()
	if err != nil {
		m.logger.Error("Failed to get active deployments", "error", err)
		return
	}

	if len(states) == 0 {
		m.logger.Debug("No active deployments to monitor")
		return
	}

	m.logger.Info("Checking for updates",
		"deployments", len(states))

	// Group by environment for efficient checking
	envMap := make(map[string][]string)
	for _, state := range states {
		envMap[state.Environment] = append(envMap[state.Environment], state.DeploymentName)
	}

	// Check each environment
	for environment, deployments := range envMap {
		if err := m.checkEnvironment(environment, deployments); err != nil {
			m.logger.Error("Failed to check environment",
				"environment", environment,
				"error", err)
		}
	}

	m.logger.Debug("Update check cycle completed")
}

// checkEnvironment checks a specific environment for updates
func (m *ImageUpdateMonitor) checkEnvironment(environment string, deploymentNames []string) error {
	m.logger.Debug("Checking environment for updates",
		"environment", environment,
		"deployments", len(deploymentNames))

	// Get repository path (assuming default repo name "lixy-cd")
	repoName := "lixy-cd"
	repoPath, err := m.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository path: %w", err)
	}

	// Get registry credentials for fetching tags
	// For now, we'll use empty token - this should be enhanced to use stored credentials
	githubToken := ""
	cred, err := m.registryService.GetCredential("ghcr")
	if err == nil && cred != nil {
		githubToken = cred.Token
	}

	// Check for updates using the reconciler
	updates, err := m.reconciler.CheckForUpdates(repoName, environment, githubToken)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if len(updates) == 0 {
		m.logger.Debug("No updates found", "environment", environment)
		return nil
	}

	m.logger.Info("Found updates",
		"environment", environment,
		"count", len(updates))

	// Process each update
	for serviceName, newTag := range updates {
		deploymentName := fmt.Sprintf("%s-%s", environment, serviceName)

		// Only process if this deployment is in our monitoring list
		isMonitored := false
		for _, name := range deploymentNames {
			if name == deploymentName {
				isMonitored = true
				break
			}
		}

		if !isMonitored {
			continue
		}

		if err := m.processUpdate(repoPath, environment, serviceName, deploymentName, newTag); err != nil {
			m.logger.Error("Failed to process update",
				"deployment", deploymentName,
				"new_tag", newTag,
				"error", err)
		}
	}

	return nil
}

// processUpdate processes a single update
func (m *ImageUpdateMonitor) processUpdate(
	repoPath string,
	environment string,
	serviceName string,
	deploymentName string,
	newTag string,
) error {
	m.logger.Info("Processing update",
		"deployment", deploymentName,
		"new_tag", newTag)

	// Get service configuration
	serviceConfig, err := m.parser.ParseServiceConfig(repoPath, environment, serviceName)
	if err != nil {
		return fmt.Errorf("failed to parse service config: %w", err)
	}

	oldTag := serviceConfig.Tag

	// Get strategy name
	strategyName := serviceConfig.Strategy
	if strategyName == "" {
		strategyName = "default"
	}

	// Record the update check
	record, err := m.updateHistory.RecordCheck(
		deploymentName,
		environment,
		serviceConfig.Image,
		oldTag,
		newTag,
		strategyName,
		true, // update found
	)
	if err != nil {
		m.logger.Error("Failed to record update check", "error", err)
		// Continue anyway
	}

	// Update monitor state
	if err := m.monitorState.UpdateCheckTime(deploymentName, environment); err != nil {
		m.logger.Error("Failed to update check time", "error", err)
	}

	// Check if auto-update is enabled (for now, we'll make it manual approval only)
	// This will be enhanced when we add auto-update configuration
	autoApply := false

	if autoApply {
		// Apply the update automatically
		if err := m.applyUpdate(repoPath, environment, serviceName, deploymentName, newTag, record.ID); err != nil {
			// Update record with failure
			m.updateHistory.UpdateStatus(record.ID, "failed", err.Error())
			return fmt.Errorf("failed to apply update: %w", err)
		}

		// Update record with success
		m.updateHistory.RecordUpdate(record.ID, true)
		m.monitorState.UpdateUpdateTime(deploymentName, environment)

		m.logger.Info("Successfully applied update",
			"deployment", deploymentName,
			"old_tag", oldTag,
			"new_tag", newTag)
	} else {
		m.logger.Info("Update requires manual approval",
			"deployment", deploymentName,
			"old_tag", oldTag,
			"new_tag", newTag,
			"update_id", record.ID)
	}

	return nil
}

// applyUpdate applies an update to a deployment
func (m *ImageUpdateMonitor) applyUpdate(
	repoPath string,
	environment string,
	serviceName string,
	deploymentName string,
	newTag string,
	updateID string,
) error {
	m.logger.Info("Applying update",
		"deployment", deploymentName,
		"new_tag", newTag)

	// Get service configuration
	serviceConfig, err := m.parser.ParseServiceConfig(repoPath, environment, serviceName)
	if err != nil {
		return fmt.Errorf("failed to parse service config: %w", err)
	}

	// Update the tag in the service configuration
	serviceConfig.Tag = newTag

	// Save the updated configuration by writing the YAML file directly
	configPath := filepath.Join(repoPath, "environments", environment, fmt.Sprintf("%s.yaml", serviceName))

	data, err := yaml.Marshal(serviceConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal service config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write service config: %w", err)
	}

	m.logger.Info("Updated service configuration file",
		"path", configPath,
		"new_tag", newTag)

	// Trigger reconciliation to apply the change
	if err := m.reconciler.ReconcileEnvironment("lixy-cd", environment, ""); err != nil {
		return fmt.Errorf("failed to reconcile environment: %w", err)
	}

	return nil
}

// CheckDeployment manually triggers an update check for a specific deployment
func (m *ImageUpdateMonitor) CheckDeployment(deploymentName, environment string) error {
	m.logger.Info("Manual update check triggered",
		"deployment", deploymentName,
		"environment", environment)

	return m.checkEnvironment(environment, []string{deploymentName})
}

// EnableDeploymentMonitoring enables monitoring for a specific deployment
func (m *ImageUpdateMonitor) EnableDeploymentMonitoring(deploymentName, environment string) error {
	if err := m.monitorState.EnableMonitoring(deploymentName, environment, true); err != nil {
		return fmt.Errorf("failed to enable monitoring: %w", err)
	}

	m.logger.Info("Enabled monitoring for deployment",
		"deployment", deploymentName,
		"environment", environment)

	return nil
}

// DisableDeploymentMonitoring disables monitoring for a specific deployment
func (m *ImageUpdateMonitor) DisableDeploymentMonitoring(deploymentName, environment string) error {
	if err := m.monitorState.EnableMonitoring(deploymentName, environment, false); err != nil {
		return fmt.Errorf("failed to disable monitoring: %w", err)
	}

	m.logger.Info("Disabled monitoring for deployment",
		"deployment", deploymentName,
		"environment", environment)

	return nil
}

// GetUpdateHistory retrieves update history for a deployment
func (m *ImageUpdateMonitor) GetUpdateHistory(deploymentName, environment string, limit int) ([]store.UpdateHistory, error) {
	return m.updateHistory.GetHistory(deploymentName, environment, limit)
}

// GetRecentUpdates retrieves recent updates across all deployments
func (m *ImageUpdateMonitor) GetRecentUpdates(limit int) ([]store.UpdateHistory, error) {
	return m.updateHistory.GetRecentUpdates(limit)
}

// GetPendingUpdates retrieves all pending updates
func (m *ImageUpdateMonitor) GetPendingUpdates() ([]store.UpdateHistory, error) {
	return m.updateHistory.GetPendingUpdates()
}

// ApproveUpdate manually approves and applies a pending update
func (m *ImageUpdateMonitor) ApproveUpdate(updateID string) error {
	// Get the update record
	record, err := m.updateHistory.GetByID(updateID)
	if err != nil {
		return fmt.Errorf("failed to get update record: %w", err)
	}

	if record.Status != "pending" {
		return fmt.Errorf("update is not pending (status: %s)", record.Status)
	}

	m.logger.Info("Approving update",
		"update_id", updateID,
		"deployment", record.DeploymentName,
		"new_tag", record.NewTag)

	// Get repository path
	repoPath, err := m.gitService.GetRepositoryPath("lixy-cd")
	if err != nil {
		return fmt.Errorf("failed to get repository path: %w", err)
	}

	// Extract service name from deployment name (format: environment-servicename)
	// This is a simplified approach - may need enhancement
	serviceName := record.DeploymentName
	if len(record.Environment) > 0 {
		prefix := record.Environment + "-"
		if len(record.DeploymentName) > len(prefix) {
			serviceName = record.DeploymentName[len(prefix):]
		}
	}

	// Apply the update
	if err := m.applyUpdate(repoPath, record.Environment, serviceName, record.DeploymentName, record.NewTag, updateID); err != nil {
		m.updateHistory.UpdateStatus(updateID, "failed", err.Error())
		return fmt.Errorf("failed to apply update: %w", err)
	}

	// Update record with success
	if err := m.updateHistory.RecordUpdate(updateID, false); err != nil {
		m.logger.Error("Failed to update history record", "error", err)
	}

	if err := m.monitorState.UpdateUpdateTime(record.DeploymentName, record.Environment); err != nil {
		m.logger.Error("Failed to update monitor state", "error", err)
	}

	m.logger.Info("Successfully approved and applied update",
		"update_id", updateID,
		"deployment", record.DeploymentName)

	return nil
}

// GetMonitorStatus returns the current status of the monitor
func (m *ImageUpdateMonitor) GetMonitorStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"enabled":          m.enabled,
		"running":          m.running,
		"default_interval": m.defaultInterval.String(),
	}
}
