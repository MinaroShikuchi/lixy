// internal/client/socket/controller.go - Controller-specific
package socket

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// ControllerClient extends the base client with controller-specific methods
type ControllerClient struct {
	*Client
}

func NewControllerClient() *ControllerClient {
	return &ControllerClient{
		Client: &Client{SocketPath: "/tmp/lixy.sock"},
	}
}

func (c *ControllerClient) DeployApplication(name string, yaml []byte) (*domain.Response, error) {
	// Higher-level method that uses SendCommand underneath
	return c.SendCommand("deploy", map[string]string{
		"name": name,
		"yaml": string(yaml),
	})
}

// ListDeployments fetches the list of deployments
func (c *Client) ListDeployments() ([]map[string]interface{}, error) {
	resp, err := c.SendCommand("list", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("command failed: %s", resp.Message)
	}

	// Parse the deployments from the response
	deployments, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	result := make([]map[string]interface{}, 0, len(deployments))
	for _, d := range deployments {
		deployment, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, deployment)
	}

	return result, nil
}

func (c *Client) GetDeployments() ([]map[string]interface{}, error) {
	resp, err := c.SendCommand("get-deployments", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("command failed: %s", resp.Message)
	}

	deployments, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	result := make([]map[string]interface{}, 0, len(deployments))
	for _, d := range deployments {
		deployment, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, deployment)
	}

	return result, nil
}

func (c *Client) GetDeployment(name string) (map[string]interface{}, error) {
	resp, err := c.SendCommand("get-deployment", map[string]string{"name": name})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("command failed: %s", resp.Message)
	}

	deployment, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	return deployment, nil
}
