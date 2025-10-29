package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

func (s *DeploymentService) DeployToTarget(name string, targetLXC string, composeYAML string) error {
	// Get agent information from the store
	agent, found := s.agentStore.Get(targetLXC)

	if !found {
		return fmt.Errorf("agent with ID %s not found", targetLXC)
	}

	// Construct the deployment endpoint
	deployURL := fmt.Sprintf("http://%s:%d/deploy", agent.IP, agent.Port)

	// Create deployment request payload
	deploymentRequest := domain.DeploymentRequest{
		Name:        name,
		ComposeYAML: []byte(composeYAML),
	}

	requestBody, err := json.Marshal(deploymentRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment request: %v", err)
	}

	// Send the deployment request to the agent
	resp, err := http.Post(deployURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to send deployment request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("deployment failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("deployment failed: %s", errorResponse.Message)
	}

	log.Printf("Deployment %s initiated successfully on target %s", name, targetLXC)
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

	log.Printf("Compose file validated successfully for target %s", targetLXC)
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
