package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
