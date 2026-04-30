package services

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"gopkg.in/yaml.v3"
)

type DeploymentService struct {
	agentStore      domain.AgentRepository
	deploymentStore domain.DeploymentRepository
}

func NewDeploymentService(agentStore domain.AgentRepository, deploymentStore domain.DeploymentRepository) *DeploymentService {
	return &DeploymentService{
		agentStore:      agentStore,
		deploymentStore: deploymentStore,
	}
}

func (s *DeploymentService) ListAllDeployments(targetLXC string) ([]domain.DeploymentDto, error) {
	storeDeployments, err := s.deploymentStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	deployments := make([]domain.DeploymentDto, 0)
	for _, d := range storeDeployments {
		if targetLXC != "" && d.TargetLXC != targetLXC {
			continue
		}
		deployments = append(deployments, domain.DeploymentDto{
			ID:          d.ID,
			Name:        d.Name,
			TargetLXC:   d.TargetLXC,
			Status:      d.Status,
			ComposeYAML: string(d.ComposeYAML),
			EnvVars:     d.EnvVars,
		})
	}
	return deployments, nil
}

// GetDeployment retrieves a single deployment by name
func (s *DeploymentService) GetDeployment(name string) (*domain.DeploymentDto, error) {
	d, exists := s.deploymentStore.Get(name)
	if !exists {
		return nil, fmt.Errorf("deployment with name '%s' not found", name)
	}
	return &domain.DeploymentDto{
		ID:          d.ID,
		Name:        d.Name,
		TargetLXC:   d.TargetLXC,
		Status:      d.Status,
		ComposeYAML: string(d.ComposeYAML),
		EnvVars:     d.EnvVars,
	}, nil
}

func (s *DeploymentService) CreateDeployment(name string, targetLXC string, composeYAML []byte, envVars map[string]string) error {
	if _, exists := s.deploymentStore.Get(name); exists {
		return fmt.Errorf("deployment with name '%s' already exists", name)
	}

	_, agentExists := s.agentStore.Get(targetLXC)
	if !agentExists {
		return fmt.Errorf("target agent '%s' not found - agent must be registered before creating deployments", targetLXC)
	}

	err := s.deploymentStore.Create(domain.DeploymentInfo{
		Name:        name,
		TargetLXC:   targetLXC,
		ComposeYAML: composeYAML,
		Status:      "pending",
		EnvVars:     envVars,
	})
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return nil
}

func (s *DeploymentService) UpdateDeployment(name string, composeYAML []byte, envVars map[string]string) error {
	if deployment, exists := s.deploymentStore.Get(name); exists {
		// Preserve existing env vars if none provided
		mergedEnvVars := deployment.EnvVars
		if envVars != nil {
			mergedEnvVars = envVars
		}
		err := s.deploymentStore.Update(domain.DeploymentInfo{
			Name:        name,
			TargetLXC:   deployment.TargetLXC,
			ComposeYAML: composeYAML,
			Status:      "pending",
			EnvVars:     mergedEnvVars,
		})
		if err != nil {
			return fmt.Errorf("failed to update deployment: %v", err)
		}
	} else {
		return fmt.Errorf("deployment with name '%s' does not exist", name)
	}

	return nil
}

func (s *DeploymentService) UpdateDeploymentStatus(name, status string) error {
	if deployment, exists := s.deploymentStore.Get(name); exists {
		err := s.deploymentStore.Update(domain.DeploymentInfo{
			Name:        name,
			TargetLXC:   deployment.TargetLXC,
			ComposeYAML: deployment.ComposeYAML,
			Status:      status,
			EnvVars:     deployment.EnvVars,
		})
		if err != nil {
			return fmt.Errorf("failed to update deployment status: %v", err)
		}
	} else {
		return fmt.Errorf("deployment with name '%s' does not exist", name)
	}

	return nil
}

func (s *DeploymentService) ValidateDeployment(targetLXC string, composeYAML string) error {
	var composeConfig map[string]interface{}

	err := yaml.Unmarshal([]byte(composeYAML), &composeConfig)
	if err != nil {
		return fmt.Errorf("invalid compose file: %v", err)
	}

	// Check for required sections
	if _, ok := composeConfig["services"]; !ok {
		return fmt.Errorf("compose file missing 'services' section")
	}

	return nil
}

// DeleteDeployment removes a deployment from the store
func (s *DeploymentService) DeleteDeployment(name string) error {
	if _, exists := s.deploymentStore.Get(name); !exists {
		return fmt.Errorf("deployment with name '%s' does not exist", name)
	}

	err := s.deploymentStore.Delete(name)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %v", err)
	}

	return nil
}

// CreateOrUpdateDeployment creates a deployment if it doesn't exist, or updates it if it does.
func (s *DeploymentService) CreateOrUpdateDeployment(name, targetLXC string, composeYAML []byte, envVars map[string]string) error {
	if _, exists := s.deploymentStore.Get(name); exists {
		return s.UpdateDeployment(name, composeYAML, envVars)
	}
	return s.CreateDeployment(name, targetLXC, composeYAML, envVars)
}

// ListDeploymentsByTarget returns deployments for a specific target agent
func (s *DeploymentService) ListDeploymentsByTarget(targetLXC string) ([]domain.DeploymentInfo, error) {
	storeDeployments, err := s.deploymentStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	deployments := make([]domain.DeploymentInfo, 0)
	for _, d := range storeDeployments {
		if d.TargetLXC == targetLXC {
			deployments = append(deployments, d)
		}
	}
	return deployments, nil
}
