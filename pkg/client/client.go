package client

import (
	"encoding/json"
	"fmt"
	"net"
)

// Command represents a request to the agent
type Command struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

// Response represents the agent's response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Client handles communication with the GitOps agent
type Client struct {
	SocketPath string
}

// NewClient creates a new client instance
func LyxiClient() *Client {
	return &Client{
		SocketPath: "/tmp/lixy.sock",
	}
}

// SendCommand sends a command to the agent and returns the response
func (c *Client) SendCommand(action string, params map[string]string) (*Response, error) {
	// Connect to the agent via Unix socket
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent: %v", err)
	}
	defer conn.Close()

	// Create and send the command
	cmd := Command{
		Action: action,
		Params: params,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("failed to send command: %v", err)
	}

	// Read and parse the response
	decoder := json.NewDecoder(conn)
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return &response, nil
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
