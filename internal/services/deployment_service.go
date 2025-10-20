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

type DeploymentRequest struct {
	Action     string            `json:"action"`
	ComposeDir string            `json:"composeDir"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
}

type DeploymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

func UpdateDeployment(targetLXC string, composeDir string) error {
	lixiesEndpoint := fmt.Sprintf("http://%s:8765/deploy", targetLXC)

	// Prepare the deployment request
	deployRequest := DeploymentRequest{
		Action:     "update",
		ComposeDir: composeDir,
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
	var deployResponse DeploymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&deployResponse); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !deployResponse.Success {
		return fmt.Errorf("deployment failed: %s", deployResponse.Message)
	}

	log.Printf("Deployment updated successfully on %s: %s", targetLXC, deployResponse.Output)
	return nil
}

func ValidateDeployment(targetLXC string, composeDir string) error {
	var composeConfig map[string]interface{}

	err := yaml.Unmarshal([]byte(composeDir), &composeConfig)
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

func DeployToTarget(name string, targetLXC string, composeYAML []byte, agentStore *store.AgentStore) error {
	// Get agent information from the store
	agent, found := agentStore.GetAgent(targetLXC)

	if !found {
		return fmt.Errorf("agent with ID %s not found", targetLXC)
	}

	// Construct the deployment endpoint
	deployURL := fmt.Sprintf("http://%s:%s/deploy", agent.IP, agent.Port)

	// Create deployment request payload
	deploymentRequest := types.DeploymentRequest{
		Name:        name,
		ComposeYAML: composeYAML,
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
