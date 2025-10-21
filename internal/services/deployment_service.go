package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/MinaroShikuchi/lixy/pkg/types"
	"gopkg.in/yaml.v3"
)

func UpdateDeployment(targetLXC string, composeYAML []byte) error {
	lixiesEndpoint := fmt.Sprintf("http://%s:8765/deploy", targetLXC)

	// Prepare the deployment request
	deployRequest := types.DeploymentRequest{
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
	var deployResponse types.DeploymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&deployResponse); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !deployResponse.Success {
		return fmt.Errorf("deployment failed: %s", deployResponse.Message)
	}

	log.Printf("Deployment updated successfully on %s: %s", targetLXC, deployResponse.Output)
	return nil
}

func ValidateDeployment(targetLXC string, composeYAML string) error {
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

func DeployToTarget(name string, targetLXC string, composeYAML string, agentStore *store.AgentStore) error {
	// Get agent information from the store
	agent, found := agentStore.GetAgent(targetLXC)

	if !found {
		return fmt.Errorf("agent with ID %s not found", targetLXC)
	}

	// Construct the deployment endpoint
	deployURL := fmt.Sprintf("http://%s:%d/deploy", agent.IP, agent.Port)

	// Create deployment request payload
	deploymentRequest := types.DeploymentRequest{
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

func DeleteDeployment(name string, targetLXC string, agentStore *store.AgentStore) error {
	// For now, just log the deletion action
	log.Printf("Deleting deployment %s from target %s", name, targetLXC)

	agent, found := agentStore.GetAgent(targetLXC)

	if !found {
		return fmt.Errorf("agent with ID %s not found", targetLXC)
	}

	// Construct the deployment endpoint
	deployURL := fmt.Sprintf("http://%s:%d/deploy", agent.IP, agent.Port)

	// Create delete request payload
	deleteRequest := types.DeploymentRequest{
		Name: name,
	}

	requestBody, err := json.Marshal(deleteRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %v", err)
	}

	// Send the delete request to the agent
	req, err := http.NewRequest(http.MethodDelete, deployURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("deletion failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("deletion failed: %s", errorResponse.Message)
	}

	log.Printf("Deployment %s deleted successfully from target %s", name, targetLXC)

	return nil
}
