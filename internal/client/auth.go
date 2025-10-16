package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type TokenData struct {
	AgentID       string    `json:"agent_id"`       // Unique identifier assigned by the controller
	Token         string    `json:"token"`          // The JWT authentication token
	IssuedAt      time.Time `json:"issued_at"`      // When the token was issued
	ControllerURL string    `json:"controller_url"` // URL of the controller this agent is registered with
}

func LoadToken() (TokenData, error) {
	// Implementation for loading token from secure storage
	configDir := "/etc/lixies"
	tokenFile := filepath.Join(configDir, "token.json")

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return TokenData{}, fmt.Errorf("failed to read token file: %w", err)
	}

	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return TokenData{}, fmt.Errorf("invalid token file format: %w", err)
	}

	return tokenData, nil
}

// func StoreToken(agentID, token, controllerURL string) error {
// 	// Create config directory if it doesn't exist
// 	configDir := "/etc/lixies"
// 	if err := os.MkdirAll(configDir, 0700); err != nil {
// 		return fmt.Errorf("failed to create config directory: %w", err)
// 	}

// 	// Create token data structure
// 	tokenData := TokenData{
// 		AgentID:       agentID,
// 		Token:         token,
// 		IssuedAt:      time.Now(),
// 		ControllerURL: controllerURL,
// 	}

// 	// Marshal to JSON
// 	jsonData, err := json.Marshal(tokenData)
// 	if err != nil {
// 		return fmt.Errorf("failed to serialize token data: %w", err)
// 	}

// 	// Write to file with restricted permissions
// 	tokenFile := filepath.Join(configDir, "token.json")
// 	if err := os.WriteFile(tokenFile, jsonData, 0600); err != nil {
// 		return fmt.Errorf("failed to write token file: %w", err)
// 	}

// 	return nil
// }

func GetToken() (TokenData, error) {
	tokenData, err := LoadToken()
	if err != nil {
		return TokenData{}, fmt.Errorf("agent not registered with controller: %w", err)
	}
	return tokenData, nil
}
