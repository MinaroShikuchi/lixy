// internal/client/socket/client.go - Base functionality
package socket

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

type Client struct {
	SocketPath string
}

// SendCommand sends a command to the agent and returns the response
func (c *Client) SendCommand(action string, params any) (*domain.Response, error) {
	// Connect to the agent via Unix socket
	conn, err := net.Dial("unix", c.SocketPath)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent: %v", err)
	}
	defer conn.Close()

	// Create and send the command

	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %v", err)
	}
	cmd := domain.Command{
		Action: action,
		Params: rawParams,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("failed to send command: %v", err)
	}

	// Read and parse the response
	decoder := json.NewDecoder(conn)
	var response domain.Response
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return &response, nil
}

func (c *Client) CreateDeployment(name string, targetLXC string, composeData []byte) error {

	// params := domain.JoinOptions{
	// 	Token:      token,
	// 	Controller: controller,
	// }

	// commandBody, err := json.Marshal(params)
	// if err != nil {
	// 	return err
	// }

	// resp, err := c.SendCommand("create-deployment", map[string]string{
	// 	"name":         name,
	// 	"target_lxc":   targetLXC,
	// 	"compose_yaml": string(composeData),
	// })

	// if !resp.Success {
	// 	return errors.New("failed to create deployment: " + resp.Message)
	// }
	// if err != nil {
	// 	return err
	// }
	return nil
}
