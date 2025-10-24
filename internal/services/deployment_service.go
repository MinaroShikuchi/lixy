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

func (s *DeploymentService) ListAllDeployments() ([]map[string]string, error) {

	deployments := make([]map[string]string, 0)
	for _, deployment := range s.deploymentStore.ListDeployments() {
		deployments = append(deployments, map[string]string{
			"name":      deployment.Name,
			"targetLXC": deployment.TargetLXC,
			"status":    deployment.Status,
		})
	}
	return deployments, nil
}

func (s *DeploymentService) CreateDeployment(name string, targetLXC string, composeYAML []byte) error {
	// Update the deployment store
	s.deploymentStore.AddOrUpdateDeployment(store.DeploymentInfo{
		Name:      name,
		TargetLXC: targetLXC,
		Status:    "running",
	})

	return nil
}

func (s *DeploymentService) DeployToTarget(name string, targetLXC string, composeYAML string) error {
	// Get agent information from the store
	agent, found := s.agentStore.GetAgent(targetLXC)

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

func (s *DeploymentService) UpdateDeployment(targetLXC string, composeYAML []byte) error {
	lixiesEndpoint := fmt.Sprintf("http://%s:8765/deploy", targetLXC)

	// Prepare the deployment request
	deployRequest := domain.DeploymentRequest{
		ComposeYAML: composeYAML,
		EnvVars: map[string]string{
			"DEPLOYMENT_ID": "app1",
		},
	}

	requestBody, err := json.Marshal(deployRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request to the lixies agent
	resp, err := http.Post(lixiesEndpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to connect to lixies agent: %v", err)
	}
	defer resp.Body.Close()

	// Parse response
	var deployResponse domain.DeploymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&deployResponse); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !deployResponse.Success {
		return fmt.Errorf("deployment failed: %s", deployResponse.Message)
	}

	log.Printf("Deployment updated successfully on %s: %s", targetLXC, deployResponse.Output)
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
	// In a real implementation, you would also notify the agent to stop/remove the deployment
	// For now, we just log the deletion
	log.Printf("Deleting deployment %s", name)

	err := s.deploymentStore.DeleteDeployment(name)
	if err != nil {
		return fmt.Errorf("failed to delete deployment from store: %v", err)
	}
	//TODO: to notify the agent to remove the deployment
	return nil
}
