package services

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// GitOpsReconciler reconciles desired state from Git with actual deployment state
type GitOpsReconciler struct {
	logger            *slog.Logger
	gitService        *GitService
	deploymentService *DeploymentService
	registryService   *RegistryService
	parser            *GitOpsParser
	agentService      *AgentService
}

// NewGitOpsReconciler creates a new GitOps reconciler
func NewGitOpsReconciler(
	logger *slog.Logger,
	gitService *GitService,
	deploymentService *DeploymentService,
	registryService *RegistryService,
	parser *GitOpsParser,
	agentService *AgentService,
) *GitOpsReconciler {
	return &GitOpsReconciler{
		logger:            logger,
		gitService:        gitService,
		deploymentService: deploymentService,
		registryService:   registryService,
		parser:            parser,
		agentService:      agentService,
	}
}

// ReconcileRepository reconciles deployments from a Git repository
func (r *GitOpsReconciler) ReconcileRepository(repoName, targetAgent string) error {
	r.logger.Info("Starting reconciliation", "repository", repoName, "target", targetAgent)

	// Get repository path
	repoPath, err := r.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository path: %w", err)
	}

	// Parse deployments from repository
	desiredDeployments, err := r.parseDeploymentsFromRepo(repoPath, targetAgent)
	if err != nil {
		return fmt.Errorf("failed to parse deployments: %w", err)
	}

	r.logger.Info("Found deployments in repository", "count", len(desiredDeployments))

	// Get current deployments from store for this target agent
	currentDeployments, err := r.deploymentService.ListDeploymentsByTarget(targetAgent)
	if err != nil {
		return fmt.Errorf("failed to list current deployments: %w", err)
	}

	r.logger.Info("Found current deployments", "count", len(currentDeployments))

	// Reconcile the differences
	if err := r.reconcileDeployments(desiredDeployments, currentDeployments, targetAgent); err != nil {
		return fmt.Errorf("failed to reconcile deployments: %w", err)
	}

	r.logger.Info("Successfully reconciled repository", "repository", repoName)
	return nil
}

// parseDeploymentsFromRepo scans the repository for docker-compose files
func (r *GitOpsReconciler) parseDeploymentsFromRepo(repoPath, targetAgent string) (map[string][]byte, error) {
	deployments := make(map[string][]byte)

	// Walk through the repository looking for docker-compose files
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Look for docker-compose.yml or docker-compose.yaml files
		if !info.IsDir() && (info.Name() == "docker-compose.yml" || info.Name() == "docker-compose.yaml") {
			// Read the compose file
			composeData, err := os.ReadFile(path)
			if err != nil {
				r.logger.Error("Failed to read compose file", "path", path, "error", err)
				return nil // Continue walking
			}

			// Use the parent directory name as the deployment name
			deploymentName := filepath.Base(filepath.Dir(path))

			// If the compose file is at the root, use the repo name
			relPath, _ := filepath.Rel(repoPath, filepath.Dir(path))
			if relPath == "." {
				deploymentName = filepath.Base(repoPath)
			}

			r.logger.Info("Found deployment",
				"name", deploymentName,
				"path", path,
				"size", len(composeData))

			deployments[deploymentName] = composeData
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	return deployments, nil
}

// reconcileDeployments reconciles desired state with current state
func (r *GitOpsReconciler) reconcileDeployments(
	desired map[string][]byte,
	current []domain.DeploymentInfo,
	targetAgent string,
) error {
	// Create a map of current deployments by name
	currentByName := make(map[string]domain.DeploymentInfo)
	for _, d := range current {
		currentByName[d.Name] = d
	}

	// Process desired deployments (create or update)
	for name, composeYAML := range desired {
		existing, exists := currentByName[name]

		if !exists {
			// Deployment doesn't exist, create it
			r.logger.Info("Creating new deployment", "name", name, "target", targetAgent)

			if err := r.deploymentService.CreateDeployment(name, targetAgent, composeYAML, nil); err != nil {
				r.logger.Error("Failed to create deployment", "name", name, "error", err)
				continue
			}

			r.logger.Info("Created deployment", "name", name)
		} else {
			// Deployment exists, check if update is needed
			if string(existing.ComposeYAML) != string(composeYAML) {
				r.logger.Info("Updating deployment", "name", name, "target", targetAgent)

				if err := r.deploymentService.UpdateDeployment(existing.ID, composeYAML, nil); err != nil {
					r.logger.Error("Failed to update deployment", "name", name, "error", err)
					continue
				}

				r.logger.Info("Updated deployment", "name", name)
			} else {
				r.logger.Debug("Deployment unchanged", "name", name)
			}

			// Remove from map so we know it's still desired
			delete(currentByName, name)
		}
	}

	// Remaining deployments in currentByName should be deleted (drift detection)
	for name, deployment := range currentByName {
		r.logger.Info("Deleting deployment (not in desired state)", "name", name)

		if err := r.deploymentService.DeleteDeployment(deployment.ID); err != nil {
			r.logger.Error("Failed to delete deployment", "name", name, "error", err)
			continue
		}

		r.logger.Info("Deleted deployment", "name", name)
	}

	return nil
}

// ReconcileWithRegistry reconciles a repository (legacy method - now just calls ReconcileRepository)
func (r *GitOpsReconciler) ReconcileWithRegistry(repoName, targetAgent, githubToken string) error {
	// Note: Registry authentication is no longer needed on the controller side
	// The controller only fetches tags via API, not pulling images
	// Agents handle image pulling and authentication
	return r.ReconcileRepository(repoName, targetAgent)
}

// ReconcileEnvironment reconciles all services in an environment using the structured approach
func (r *GitOpsReconciler) ReconcileEnvironment(repoName, environment string, githubToken string) error {
	r.logger.Info("Starting environment reconciliation",
		"repository", repoName,
		"environment", environment)

	// Get repository path
	repoPath, err := r.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository path: %w", err)
	}

	// Parse environment configuration
	envConfig, err := r.parser.ParseEnvironmentConfig(repoPath, environment)
	if err != nil {
		return fmt.Errorf("failed to parse environment config: %w", err)
	}

	r.logger.Info("Loaded environment configuration",
		"environment", envConfig.Name,
		"variables", len(envConfig.Variables))

	// List all services in the environment
	services, err := r.parser.ListServicesInEnvironment(repoPath, environment)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	r.logger.Info("Found services in environment",
		"environment", environment,
		"count", len(services))

	// Reconcile each service
	for _, serviceName := range services {
		if err := r.reconcileService(repoPath, environment, serviceName, envConfig); err != nil {
			r.logger.Error("Failed to reconcile service",
				"service", serviceName,
				"environment", environment,
				"error", err)
			// Continue with other services
			continue
		}
	}

	r.logger.Info("Successfully reconciled environment",
		"repository", repoName,
		"environment", environment)

	return nil
}

// reconcileService reconciles a single service
func (r *GitOpsReconciler) reconcileService(
	repoPath string,
	environment string,
	serviceName string,
	envConfig *EnvironmentConfig,
) error {
	r.logger.Info("Reconciling service",
		"service", serviceName,
		"environment", environment)

	// Parse service configuration
	serviceConfig, err := r.parser.ParseServiceConfig(repoPath, environment, serviceName)
	if err != nil {
		return fmt.Errorf("failed to parse service config: %w", err)
	}

	// Skip if service is disabled
	if !serviceConfig.Enabled {
		r.logger.Info("Service is disabled, skipping",
			"service", serviceName)
		return nil
	}

	// Merge environment and service variables
	variables := r.parser.MergeVariables(envConfig.Variables, serviceConfig.Variables)

	// Add computed variables
	variables["SERVICE_NAME"] = serviceName
	variables["ENVIRONMENT"] = environment
	variables["IMAGE"] = serviceConfig.Image
	variables["TAG"] = serviceConfig.Tag

	// Find and render the template
	templatePath := filepath.Join(repoPath, "templates", serviceName, "docker-compose.yml.gotmpl")
	composeYAML, err := r.parser.RenderTemplate(templatePath, variables)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Calculate hash of the rendered compose file
	hash := fmt.Sprintf("%x", sha256.Sum256(composeYAML))

	// Get current state
	currentState, err := r.parser.ParseServiceState(repoPath, environment, serviceName)
	if err != nil {
		r.logger.Warn("Failed to parse service state, treating as new deployment",
			"service", serviceName,
			"error", err)
		currentState = &ServiceState{
			Name:   serviceName,
			Status: "not-deployed",
		}
	}

	// Check if update is needed
	needsUpdate := currentState.DeploymentHash != hash ||
		currentState.Image != serviceConfig.Image ||
		currentState.Tag != serviceConfig.Tag

	targetAgent := serviceConfig.TargetAgent
	if targetAgent == "" {
		return fmt.Errorf("target_agent not specified for service %s", serviceName)
	}

	deploymentName := fmt.Sprintf("%s-%s", environment, serviceName)

	if currentState.Status == "not-deployed" {
		// Create new deployment
		r.logger.Info("Creating new deployment",
			"service", serviceName,
			"target", targetAgent)

		if err := r.deploymentService.CreateDeployment(deploymentName, targetAgent, composeYAML, nil); err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		// Update state
		newState := &ServiceState{
			Name:            serviceName,
			Image:           serviceConfig.Image,
			Tag:             serviceConfig.Tag,
			DeployedAt:      time.Now().Format(time.RFC3339),
			DeploymentHash:  hash,
			Variables:       variables,
			Status:          "deployed",
			LastReconcileAt: time.Now().Format(time.RFC3339),
		}

		if err := r.parser.SaveServiceState(repoPath, environment, newState); err != nil {
			r.logger.Error("Failed to save service state", "error", err)
		}

		r.logger.Info("Created deployment", "service", serviceName)

	} else if needsUpdate {
		// Update existing deployment
		r.logger.Info("Updating deployment",
			"service", serviceName,
			"old_hash", currentState.DeploymentHash,
			"new_hash", hash)

		if err := r.deploymentService.UpdateDeployment(deploymentName, composeYAML, nil); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}

		// Update state
		currentState.Image = serviceConfig.Image
		currentState.Tag = serviceConfig.Tag
		currentState.DeploymentHash = hash
		currentState.Variables = variables
		currentState.LastReconcileAt = time.Now().Format(time.RFC3339)

		if err := r.parser.SaveServiceState(repoPath, environment, currentState); err != nil {
			r.logger.Error("Failed to save service state", "error", err)
		}

		r.logger.Info("Updated deployment", "service", serviceName)

	} else {
		// No update needed
		r.logger.Debug("Service is up to date", "service", serviceName)

		// Update last reconcile time
		currentState.LastReconcileAt = time.Now().Format(time.RFC3339)
		if err := r.parser.SaveServiceState(repoPath, environment, currentState); err != nil {
			r.logger.Error("Failed to save service state", "error", err)
		}
	}

	return nil
}

// DetectDrift detects configuration drift for an environment
func (r *GitOpsReconciler) DetectDrift(repoName, environment string) ([]string, error) {
	repoPath, err := r.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository path: %w", err)
	}

	services, err := r.parser.ListServicesInEnvironment(repoPath, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	driftedServices := make([]string, 0)

	for _, serviceName := range services {
		state, err := r.parser.ParseServiceState(repoPath, environment, serviceName)
		if err != nil {
			continue
		}

		// Get current deployment
		deploymentName := fmt.Sprintf("%s-%s", environment, serviceName)
		currentDeployments, err := r.deploymentService.ListDeploymentsByTarget("")
		if err != nil {
			continue
		}

		found := false
		for _, d := range currentDeployments {
			if d.Name == deploymentName {
				found = true
				// Check if compose YAML matches
				currentHash := fmt.Sprintf("%x", sha256.Sum256(d.ComposeYAML))
				if currentHash != state.DeploymentHash {
					driftedServices = append(driftedServices, serviceName)
					r.logger.Warn("Drift detected",
						"service", serviceName,
						"expected_hash", state.DeploymentHash,
						"actual_hash", currentHash)
				}
				break
			}
		}

		if !found && state.Status == "deployed" {
			driftedServices = append(driftedServices, serviceName)
			r.logger.Warn("Drift detected - deployment missing",
				"service", serviceName)
		}
	}

	return driftedServices, nil
}

// CheckForUpdates checks if newer versions are available for services in an environment
func (r *GitOpsReconciler) CheckForUpdates(repoName, environment, githubToken string) (map[string]string, error) {
	repoPath, err := r.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository path: %w", err)
	}

	services, err := r.parser.ListServicesInEnvironment(repoPath, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	updates := make(map[string]string)

	for _, serviceName := range services {
		serviceConfig, err := r.parser.ParseServiceConfig(repoPath, environment, serviceName)
		if err != nil {
			r.logger.Error("Failed to parse service config", "service", serviceName, "error", err)
			continue
		}

		if !serviceConfig.Enabled {
			continue
		}

		// Fetch available tags from registry
		tags, err := r.registryService.FetchTags(serviceConfig.Image, githubToken)
		if err != nil {
			r.logger.Error("Failed to fetch tags",
				"service", serviceName,
				"image", serviceConfig.Image,
				"error", err)
			continue
		}

		// Apply version strategy if specified
		if serviceConfig.Strategy != "" {
			strategy, err := r.parser.ParseVersionStrategy(repoPath, serviceConfig.Strategy)
			if err != nil {
				r.logger.Warn("Failed to parse strategy, using all tags",
					"service", serviceName,
					"strategy", serviceConfig.Strategy,
					"error", err)
			} else {
				tags = r.registryService.FilterTagsByStrategy(tags, strategy)
			}
		}

		// Get latest tag
		latestTag := r.registryService.GetLatestTag(tags)

		// Check if there's a newer version
		if latestTag != "" && latestTag != serviceConfig.Tag {
			r.logger.Info("Update available",
				"service", serviceName,
				"current", serviceConfig.Tag,
				"latest", latestTag)
			updates[serviceName] = latestTag
		}
	}

	return updates, nil
}

// ReconcileEnvironmentDeployments reconciles deployments defined directly in environment YAML files
func (r *GitOpsReconciler) ReconcileEnvironmentDeployments(repoName, environment string) error {
	r.logger.Info("Starting environment deployment reconciliation",
		"repository", repoName,
		"environment", environment)

	// Get repository path
	repoPath, err := r.gitService.GetRepositoryPath(repoName)
	if err != nil {
		return fmt.Errorf("failed to get repository path: %w", err)
	}

	// Parse environment configuration
	envConfig, err := r.parser.ParseEnvironmentConfig(repoPath, environment)
	if err != nil {
		return fmt.Errorf("failed to parse environment config: %w", err)
	}

	// Parse deployment configurations from environment directory
	deployments, err := r.parser.ParseEnvironmentDeployments(repoPath, environment)
	if err != nil {
		return fmt.Errorf("failed to parse environment deployments: %w", err)
	}

	r.logger.Info("Found deployments in environment",
		"environment", environment,
		"count", len(deployments))

	// Process each deployment
	for _, deployment := range deployments {
		if !deployment.Enabled {
			r.logger.Info("Deployment is disabled, skipping",
				"deployment", deployment.Name)
			continue
		}

		if err := r.reconcileEnvironmentDeployment(repoPath, environment, deployment, envConfig); err != nil {
			r.logger.Error("Failed to reconcile deployment",
				"deployment", deployment.Name,
				"environment", environment,
				"error", err)
			// Continue with other deployments
			continue
		}
	}

	r.logger.Info("Successfully reconciled environment deployments",
		"repository", repoName,
		"environment", environment)

	return nil
}

// reconcileEnvironmentDeployment reconciles a single deployment from environment YAML
func (r *GitOpsReconciler) reconcileEnvironmentDeployment(
	repoPath string,
	environment string,
	deployment *DeploymentConfig,
	envConfig *EnvironmentConfig,
) error {
	r.logger.Info("Reconciling environment deployment",
		"deployment", deployment.Name,
		"environment", environment,
		"target", deployment.TargetAgent)

	// Determine target agent (auto-select if not specified)
	targetAgent := deployment.TargetAgent
	if targetAgent == "" {
		selectedAgent, err := r.selectAvailableAgent()
		if err != nil {
			return fmt.Errorf("failed to select target agent: %w", err)
		}
		targetAgent = selectedAgent
		r.logger.Info("Auto-selected target agent",
			"deployment", deployment.Name,
			"agent", targetAgent)
	}

	// Generate Docker Compose YAML
	composeYAML, err := r.parser.GenerateDeploymentComposeYAML(repoPath, environment, deployment, envConfig)
	if err != nil {
		return fmt.Errorf("failed to generate compose YAML: %w", err)
	}

	// Create or update the deployment
	deploymentName := fmt.Sprintf("%s-%s", environment, deployment.Name)

	// Check if deployment already exists
	existingDeployments, err := r.deploymentService.ListAllDeployments("")
	if err != nil {
		return fmt.Errorf("failed to list existing deployments: %w", err)
	}

	deploymentExists := false
	for _, existing := range existingDeployments {
		if existing.Name == deploymentName {
			deploymentExists = true
			break
		}
	}

	if deploymentExists {
		// Update existing deployment
		err = r.deploymentService.UpdateDeployment(deploymentName, composeYAML, nil)
		if err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		r.logger.Info("Updated deployment",
			"deployment", deploymentName,
			"target", deployment.TargetAgent)
	} else {
		// Create new deployment
		err = r.deploymentService.CreateDeployment(deploymentName, targetAgent, composeYAML, nil)
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		r.logger.Info("Created deployment",
			"deployment", deploymentName,
			"target", targetAgent)
	}

	return nil
}

// selectAvailableAgent selects an available agent for deployment
// Selection strategy: pick the agent with the fewest deployments
func (r *GitOpsReconciler) selectAvailableAgent() (string, error) {
	agents := r.agentService.ListAgents()

	if len(agents) == 0 {
		return "", fmt.Errorf("no agents available - at least one agent must be registered")
	}

	// Get all deployments to count per agent
	allDeployments, err := r.deploymentService.ListAllDeployments("")
	if err != nil {
		r.logger.Warn("Failed to list deployments for agent selection, using first available agent", "error", err)
		return agents[0].Name, nil
	}

	// Count deployments per agent
	deploymentCounts := make(map[string]int)
	for _, agent := range agents {
		deploymentCounts[agent.Name] = 0
	}
	for _, deployment := range allDeployments {
		if _, exists := deploymentCounts[deployment.TargetLXC]; exists {
			deploymentCounts[deployment.TargetLXC]++
		}
	}

	// Find agent with minimum deployments
	var selectedAgent string
	minDeployments := -1
	for _, agent := range agents {
		count := deploymentCounts[agent.Name]
		if minDeployments == -1 || count < minDeployments {
			minDeployments = count
			selectedAgent = agent.Name
		}
	}

	return selectedAgent, nil
}
