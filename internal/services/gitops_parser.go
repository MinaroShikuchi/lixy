package services

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

// GitOpsParser parses GitOps repository structure
type GitOpsParser struct {
	logger *slog.Logger
}

// NewGitOpsParser creates a new GitOps parser
func NewGitOpsParser(logger *slog.Logger) *GitOpsParser {
	return &GitOpsParser{
		logger: logger,
	}
}

// EnvironmentConfig represents environment-wide configuration
type EnvironmentConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
	Registry    RegistryConfig    `yaml:"registry"`
}

// RegistryConfig represents container registry configuration
type RegistryConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	UseToken bool   `yaml:"use_token"` // Whether to use GitHub token for auth
}

// ServiceConfig represents service-specific configuration
type ServiceConfig struct {
	Name         string            `yaml:"name"`
	Enabled      bool              `yaml:"enabled"`
	Image        string            `yaml:"image"`
	Tag          string            `yaml:"tag"`
	Strategy     string            `yaml:"strategy"`     // Reference to strategy file
	Variables    map[string]string `yaml:"variables"`    // Service-specific variables
	TargetAgent  string            `yaml:"target_agent"` // Which agent to deploy to
	Dependencies []string          `yaml:"dependencies"` // Service dependencies
}

// VersionStrategy represents a version update strategy
// Supports multiple rule types that can be combined:
//   - semver: Controls semantic version increments (major/minor/patch)
//   - stability: Controls prerelease and RC versions
//   - pattern: Enforces regex pattern matching on version strings
//   - fixed-versions: Restricts to specific allowed versions
//   - schedule: Defines time windows for updates (evaluated separately)
type VersionStrategy struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Rules       []StrategyRule `yaml:"rules"`
}

// StrategyRule represents a single rule in a strategy
// Different rule types use different fields:
//   - semver: AllowMajor, AllowMinor, AllowPatch
//   - stability: AllowPrerelease, AllowRCs
//   - pattern: Pattern, Required
//   - fixed-versions: AllowedVersions
//   - schedule: TimeWindows
type StrategyRule struct {
	Type string `yaml:"type"` // semver, stability, pattern, schedule, fixed-versions

	// Semver rule fields
	AllowMajor *bool `yaml:"allowMajor,omitempty"`
	AllowMinor *bool `yaml:"allowMinor,omitempty"`
	AllowPatch *bool `yaml:"allowPatch,omitempty"`

	// Stability rule fields
	AllowPrerelease *bool `yaml:"allowPrerelease,omitempty"`
	AllowRCs        *bool `yaml:"allowRCs,omitempty"`

	// Pattern rule fields
	Pattern  string `yaml:"pattern,omitempty"`
	Required *bool  `yaml:"required,omitempty"`

	// Fixed versions rule fields
	AllowedVersions []string `yaml:"allowedVersions,omitempty"`

	// Schedule rule fields
	TimeWindows []TimeWindow `yaml:"timeWindows,omitempty"`
}

// TimeWindow represents a time window for scheduled updates
type TimeWindow struct {
	Days  []string `yaml:"days"`  // e.g., ["Mon", "Wed", "Fri"]
	Hours []string `yaml:"hours"` // e.g., ["9-17", "10-14"]
}

// ServiceState represents the current deployed state of a service
type ServiceState struct {
	Name            string            `yaml:"name"`
	Image           string            `yaml:"image"`
	Tag             string            `yaml:"tag"`
	DeployedAt      string            `yaml:"deployed_at"`
	DeploymentHash  string            `yaml:"deployment_hash"` // Hash of compose file
	Variables       map[string]string `yaml:"variables"`
	Status          string            `yaml:"status"`
	LastReconcileAt string            `yaml:"last_reconcile_at"`
}

// ControllerConfig represents controller-wide configuration
type ControllerConfig struct {
	ReconcileInterval    string             `yaml:"reconcile_interval"` // e.g., "5m"
	EnableDriftDetection bool               `yaml:"enable_drift_detection"`
	EnableAutoUpdate     bool               `yaml:"enable_auto_update"`
	Notifications        NotificationConfig `yaml:"notifications"`
}

// NotificationConfig represents notification settings
type NotificationConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Webhooks []string `yaml:"webhooks"`
}

// DriftDetectionConfig represents drift detection configuration
type DriftDetectionConfig struct {
	Enabled           bool     `yaml:"enabled"`
	CheckInterval     string   `yaml:"check_interval"`
	AutoRemediate     bool     `yaml:"auto_remediate"`
	IgnorePaths       []string `yaml:"ignore_paths"`
	ProtectedServices []string `yaml:"protected_services"`
}

// ParseEnvironmentConfig parses environment configuration
func (p *GitOpsParser) ParseEnvironmentConfig(repoPath, environment string) (*EnvironmentConfig, error) {
	configPath := filepath.Join(repoPath, "environments", environment, "config.yaml")

	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// If no separate environment config exists, return default config
		p.logger.Info("No environment config file found, using defaults",
			"environment", environment,
			"expected_path", configPath)

		return &EnvironmentConfig{
			Name:        environment,
			Description: fmt.Sprintf("Environment: %s", environment),
			Variables:   make(map[string]string),
			Registry:    RegistryConfig{},
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment config: %w", err)
	}

	var config EnvironmentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse environment config: %w", err)
	}

	p.logger.Info("Parsed environment config",
		"environment", environment,
		"name", config.Name)

	return &config, nil
}

// ParseServiceConfig parses service-specific configuration
func (p *GitOpsParser) ParseServiceConfig(repoPath, environment, serviceName string) (*ServiceConfig, error) {
	// Services are defined directly in the environment directory, not in a services/ subdirectory
	configPath := filepath.Join(repoPath, "environments", environment, fmt.Sprintf("%s.yaml", serviceName))

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service config: %w", err)
	}

	var config ServiceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse service config: %w", err)
	}

	// Set name if not specified
	if config.Name == "" {
		config.Name = serviceName
	}

	p.logger.Info("Parsed service config",
		"environment", environment,
		"service", serviceName,
		"enabled", config.Enabled)

	return &config, nil
}

// ParseVersionStrategy parses a version strategy
func (p *GitOpsParser) ParseVersionStrategy(repoPath, strategyName string) (*VersionStrategy, error) {
	strategyPath := filepath.Join(repoPath, "strategies", fmt.Sprintf("%s.yaml", strategyName))

	data, err := os.ReadFile(strategyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategy: %w", err)
	}

	var strategy VersionStrategy
	if err := yaml.Unmarshal(data, &strategy); err != nil {
		return nil, fmt.Errorf("failed to parse strategy: %w", err)
	}

	p.logger.Info("Parsed version strategy",
		"strategy", strategyName,
		"name", strategy.Name)

	return &strategy, nil
}

// ParseServiceState parses the current deployed state of a service
func (p *GitOpsParser) ParseServiceState(repoPath, environment, serviceName string) (*ServiceState, error) {
	statePath := filepath.Join(repoPath, "state", environment, fmt.Sprintf("%s.yaml", serviceName))

	// If state file doesn't exist, return empty state
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return &ServiceState{
			Name:   serviceName,
			Status: "not-deployed",
		}, nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service state: %w", err)
	}

	var state ServiceState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse service state: %w", err)
	}

	return &state, nil
}

// SaveServiceState saves the current state of a service
func (p *GitOpsParser) SaveServiceState(repoPath, environment string, state *ServiceState) error {
	stateDir := filepath.Join(repoPath, "state", environment)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, fmt.Sprintf("%s.yaml", state.Name))

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal service state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write service state: %w", err)
	}

	p.logger.Info("Saved service state",
		"environment", environment,
		"service", state.Name,
		"status", state.Status)

	return nil
}

// ParseControllerConfig parses controller configuration
func (p *GitOpsParser) ParseControllerConfig(repoPath string) (*ControllerConfig, error) {
	configPath := filepath.Join(repoPath, "controller", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read controller config: %w", err)
	}

	var config ControllerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse controller config: %w", err)
	}

	return &config, nil
}

// ParseDriftDetectionConfig parses drift detection configuration
func (p *GitOpsParser) ParseDriftDetectionConfig(repoPath string) (*DriftDetectionConfig, error) {
	configPath := filepath.Join(repoPath, "controller", "drift-detection.yaml")

	// Return default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &DriftDetectionConfig{
			Enabled:       false,
			CheckInterval: "5m",
			AutoRemediate: false,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read drift detection config: %w", err)
	}

	var config DriftDetectionConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse drift detection config: %w", err)
	}

	return &config, nil
}

// RenderTemplate renders a Go template with variables
func (p *GitOpsParser) RenderTemplate(templatePath string, variables map[string]string) ([]byte, error) {
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(templateData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	p.logger.Debug("Rendered template",
		"template", templatePath,
		"variables", len(variables))

	return buf.Bytes(), nil
}

// ListServicesInEnvironment lists all services configured for an environment
func (p *GitOpsParser) ListServicesInEnvironment(repoPath, environment string) ([]string, error) {
	// Services are defined directly in the environment directory, not in a services/ subdirectory
	environmentDir := filepath.Join(repoPath, "environments", environment)

	entries, err := os.ReadDir(environmentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory: %w", err)
	}

	services := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			// Remove .yaml extension to get service name
			serviceName := entry.Name()[:len(entry.Name())-5]
			services = append(services, serviceName)
		}
	}

	p.logger.Info("Listed services in environment",
		"environment", environment,
		"count", len(services))

	return services, nil
}

// MergeVariables merges environment and service variables, with service taking precedence
func (p *GitOpsParser) MergeVariables(envVars, serviceVars map[string]string) map[string]string {
	merged := make(map[string]string)

	// Add environment variables
	for k, v := range envVars {
		merged[k] = v
	}

	// Override with service variables
	for k, v := range serviceVars {
		merged[k] = v
	}

	return merged
}

// GetSemverRule returns the semver rule from a strategy, if it exists
func (s *VersionStrategy) GetSemverRule() *StrategyRule {
	for i := range s.Rules {
		if s.Rules[i].Type == "semver" {
			return &s.Rules[i]
		}
	}
	return nil
}

// GetStabilityRule returns the stability rule from a strategy, if it exists
func (s *VersionStrategy) GetStabilityRule() *StrategyRule {
	for i := range s.Rules {
		if s.Rules[i].Type == "stability" {
			return &s.Rules[i]
		}
	}
	return nil
}

// GetPatternRule returns the pattern rule from a strategy, if it exists
func (s *VersionStrategy) GetPatternRule() *StrategyRule {
	for i := range s.Rules {
		if s.Rules[i].Type == "pattern" {
			return &s.Rules[i]
		}
	}
	return nil
}

// GetFixedVersionsRule returns the fixed-versions rule from a strategy, if it exists
func (s *VersionStrategy) GetFixedVersionsRule() *StrategyRule {
	for i := range s.Rules {
		if s.Rules[i].Type == "fixed-versions" {
			return &s.Rules[i]
		}
	}
	return nil
}

// GetScheduleRule returns the schedule rule from a strategy, if it exists
func (s *VersionStrategy) GetScheduleRule() *StrategyRule {
	for i := range s.Rules {
		if s.Rules[i].Type == "schedule" {
			return &s.Rules[i]
		}
	}
	return nil
}

// AllowsMajorUpdates checks if the strategy allows major version updates
func (s *VersionStrategy) AllowsMajorUpdates() bool {
	rule := s.GetSemverRule()
	if rule == nil || rule.AllowMajor == nil {
		return false
	}
	return *rule.AllowMajor
}

// AllowsMinorUpdates checks if the strategy allows minor version updates
func (s *VersionStrategy) AllowsMinorUpdates() bool {
	rule := s.GetSemverRule()
	if rule == nil || rule.AllowMinor == nil {
		return false
	}
	return *rule.AllowMinor
}

// AllowsPatchUpdates checks if the strategy allows patch version updates
func (s *VersionStrategy) AllowsPatchUpdates() bool {
	rule := s.GetSemverRule()
	if rule == nil || rule.AllowPatch == nil {
		return false
	}
	return *rule.AllowPatch
}

// AllowsPrerelease checks if the strategy allows prerelease versions
func (s *VersionStrategy) AllowsPrerelease() bool {
	rule := s.GetStabilityRule()
	if rule == nil || rule.AllowPrerelease == nil {
		return false
	}
	return *rule.AllowPrerelease
}

// AllowsRCs checks if the strategy allows release candidates
func (s *VersionStrategy) AllowsRCs() bool {
	rule := s.GetStabilityRule()
	if rule == nil || rule.AllowRCs == nil {
		return false
	}
	return *rule.AllowRCs
}

// GetAllowedVersions returns the list of allowed versions from fixed-versions rule
func (s *VersionStrategy) GetAllowedVersions() []string {
	rule := s.GetFixedVersionsRule()
	if rule == nil {
		return nil
	}
	return rule.AllowedVersions
}

// GetVersionPattern returns the version pattern from pattern rule
func (s *VersionStrategy) GetVersionPattern() (string, bool) {
	rule := s.GetPatternRule()
	if rule == nil {
		return "", false
	}
	required := rule.Required != nil && *rule.Required
	return rule.Pattern, required
}

// DeploymentConfig represents a deployment definition in environment YAML
type DeploymentConfig struct {
	Name        string            `yaml:"name"`
	Enabled     bool              `yaml:"enabled"`
	Template    string            `yaml:"template"`     // Path to template file
	Strategy    string            `yaml:"strategy"`     // Version update strategy
	Image       string            `yaml:"image"`        // Docker image
	Version     string            `yaml:"version"`      // Image version/tag
	Variables   map[string]string `yaml:"variables"`    // Template variables
	TargetAgent string            `yaml:"target_agent"` // Target LXC agent
	ComposeYAML string            `yaml:"compose_yaml"` // Direct Docker Compose YAML (alternative to template)
}

// ParseEnvironmentDeployments parses deployment definitions directly from environment YAML files
func (p *GitOpsParser) ParseEnvironmentDeployments(repoPath, environment string) ([]*DeploymentConfig, error) {
	environmentDir := filepath.Join(repoPath, "environments", environment)

	entries, err := os.ReadDir(environmentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory: %w", err)
	}

	deployments := make([]*DeploymentConfig, 0)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		deploymentPath := filepath.Join(environmentDir, entry.Name())
		p.logger.Info("Processing environment file",
			"environment", environment,
			"file", entry.Name(),
			"path", deploymentPath)

		// Try to parse each YAML file as a deployment definition
		// This includes config.yaml and any other *.yaml files
		data, err := os.ReadFile(deploymentPath)
		if err != nil {
			p.logger.Warn("Failed to read deployment file",
				"path", deploymentPath,
				"error", err)
			continue
		}

		var deployment DeploymentConfig
		if err := yaml.Unmarshal(data, &deployment); err != nil {
			p.logger.Warn("Failed to parse deployment YAML",
				"path", deploymentPath,
				"error", err)
			continue
		}

		// Set default values
		if deployment.Name == "" {
			// Use filename without extension as deployment name
			deployment.Name = entry.Name()[:len(entry.Name())-5]
		}

		// Only process if we have a valid deployment with a name
		if deployment.Name != "" {
			// Default enabled to true if not explicitly set in YAML
			// This handles cases where the enabled field is missing from the YAML
			if deployment.Name != "" && !deployment.Enabled && deployment.Template != "" {
				deployment.Enabled = true
			}

			p.logger.Info("Parsed deployment from environment file",
				"file", entry.Name(),
				"deployment", deployment.Name,
				"template", deployment.Template,
				"enabled", deployment.Enabled)

			deployments = append(deployments, &deployment)
			p.logger.Info("Parsed deployment config",
				"deployment", deployment.Name,
				"enabled", deployment.Enabled,
				"target", deployment.TargetAgent)
		}
	}

	return deployments, nil
}

// GenerateDeploymentComposeYAML generates Docker Compose YAML for a deployment
func (p *GitOpsParser) GenerateDeploymentComposeYAML(
	repoPath string,
	environment string,
	deployment *DeploymentConfig,
	envConfig *EnvironmentConfig,
) ([]byte, error) {
	// If deployment has direct compose YAML, use it
	if deployment.ComposeYAML != "" {
		return []byte(deployment.ComposeYAML), nil
	}

	// Otherwise, render template
	if deployment.Template == "" {
		return nil, fmt.Errorf("deployment %s has no template or compose_yaml specified", deployment.Name)
	}

	// Merge environment and deployment variables
	envVars := make(map[string]string)
	if envConfig != nil && envConfig.Variables != nil {
		envVars = envConfig.Variables
	}

	deploymentVars := make(map[string]string)
	if deployment.Variables != nil {
		deploymentVars = deployment.Variables
	}

	variables := p.MergeVariables(envVars, deploymentVars)

	// Add computed variables (using template-expected names)
	variables["serviceName"] = deployment.Name
	variables["deploymentName"] = deployment.Name // Alternative name
	variables["environment"] = environment
	if deployment.Image != "" {
		variables["image"] = deployment.Image
	}
	if deployment.Version != "" {
		variables["version"] = deployment.Version
		variables["tag"] = deployment.Version // Alternative name
	}

	// Render template
	templatePath := filepath.Join(repoPath, "templates", deployment.Template)
	p.logger.Info("Rendering deployment template",
		"deployment", deployment.Name,
		"template", deployment.Template,
		"templatePath", templatePath)
	return p.RenderTemplate(templatePath, variables)
}
