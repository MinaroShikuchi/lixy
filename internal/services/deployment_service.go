package services

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/store"
	"gopkg.in/yaml.v3"
)

type DeploymentService struct {
	agentStore      *store.AgentStore
	deploymentStore *store.DeploymentStore
}

func NewDeploymentService(agentStore *store.AgentStore, deploymentStore *store.DeploymentStore) *DeploymentService {
	return &DeploymentService{
		agentStore:      agentStore,
		deploymentStore: deploymentStore,
	}
}

func (s *DeploymentService) ListAllDeployments(targetLXC string) ([]domain.DeploymentDto, error) {
	deployments := make([]domain.DeploymentDto, 0)
	for _, d := range s.deploymentStore.List() {
		if targetLXC != "" && d.TargetLXC != targetLXC {
			continue
		}
		deployments = append(deployments, domain.DeploymentDto{
			ID:          d.ID,
			Name:        d.Name,
			TargetLXC:   d.TargetLXC,
			Status:      d.Status,
			ComposeYAML: d.ComposeYAML,
		})
	}
	return deployments, nil
}

func (s *DeploymentService) CreateDeployment(name string, targetLXC string, composeYAML []byte) error {
	// Update the deployment store
	if _, exists := s.deploymentStore.Get(name); exists {
		return fmt.Errorf("deployment with name '%s' already exists", name)
	}

	// Verify that the target agent exists
	_, agentExists := s.agentStore.Get(targetLXC)
	if !agentExists {
		return fmt.Errorf("target agent '%s' not found - agent must be registered before creating deployments", targetLXC)
	}

	err := s.deploymentStore.Create(store.DeploymentInfo{
		Name:        name,
		TargetLXC:   targetLXC,
		ComposeYAML: composeYAML,
		Status:      "pending",
	})
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return nil
}

func (s *DeploymentService) UpdateDeployment(name string, composeYAML []byte) error {
	if deployment, exists := s.deploymentStore.Get(name); exists {
		err := s.deploymentStore.Update(store.DeploymentInfo{
			Name:        name,
			TargetLXC:   deployment.TargetLXC,
			ComposeYAML: composeYAML,
			Status:      "pending",
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
		err := s.deploymentStore.Update(store.DeploymentInfo{
			Name:        name,
			TargetLXC:   deployment.TargetLXC,
			ComposeYAML: deployment.ComposeYAML,
			Status:      status,
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

// ListDeploymentsByTarget returns deployments for a specific target agent
func (s *DeploymentService) ListDeploymentsByTarget(targetLXC string) ([]store.DeploymentInfo, error) {
	deployments := make([]store.DeploymentInfo, 0)
	for _, d := range s.deploymentStore.List() {
		if d.TargetLXC == targetLXC {
			deployments = append(deployments, d)
		}
	}
	return deployments, nil
}
