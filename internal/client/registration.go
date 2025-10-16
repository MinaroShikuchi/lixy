// internal/client/registration.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/store"
)

// RegisterWithController registers this agent with the controller using the provided token
func RegisterWithController(tokenStore store.TokenStore, controllerURL, registrationToken, agentName string) error {
	// Get system information
	sysInfo, err := GetSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	//TODO: Get information about network interfaces and IP addresses from the socket with lixies
	// Prepare registration request
	reqBody, err := json.Marshal(map[string]interface{}{
		"token":      registrationToken,
		"agent_name": agentName,
		"agent_info": sysInfo,
		"version":    Version,
		"ip":         "localhost", // Using hostname as a placeholder for IP
		"port":       8765,        // Default port for lixy agent
	})

	if err != nil {
		return fmt.Errorf("error preparing registration request: %w", err)
	}

	// Send registration request
	resp, err := http.Post(
		fmt.Sprintf("%s/api/register-agent", controllerURL),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("error connecting to controller: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s", body)
	}

	// Parse response
	var regResp struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("error parsing registration response: %w", err)
	}
	if err := tokenStore.StoreToken(regResp.AgentID, regResp.Token, controllerURL); err != nil {
		return fmt.Errorf("error storing permanent token: %w", err)
	}

	log.Printf("Successfully registered with controller as agent %s", regResp.AgentID)
	return nil
}
