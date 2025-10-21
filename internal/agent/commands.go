// internal/agent/commands.go
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/shared"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

type AgentCommandHandler struct {
	Logger     *slog.Logger
	TokenStore *store.TokenStore
}

func (ch *AgentCommandHandler) HandleCommand(cmd domain.Command) domain.Response {
	switch cmd.Action {
	case "register-agent":
		return ch.handleRegisterAgent(cmd)
	default:
		return domain.Response{Success: false, Message: "Unknown command"}
	}
}

// Command-specific handlers
func (ch *AgentCommandHandler) handleRegisterAgent(cmd domain.Command) domain.Response {
	if err := ch.registerWithController(cmd.Params["controller"], cmd.Params["token"], cmd.Params["name"]); err != nil {
		ch.Logger.Error("Agent registration failed", "error", err)
		return domain.Response{Success: false, Message: "Registration failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: "Agent registered successfully"}
}

// RegisterWithController registers this agent with the controller using the provided token
func (ch *AgentCommandHandler) registerWithController(controllerURL, registrationToken, agentName string) error {
	// Get system information
	sysInfo, err := shared.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	//TODO: Get information about network interfaces and IP addresses from the socket with lixies
	// Prepare registration request
	reqBody, err := json.Marshal(map[string]interface{}{
		"token":      registrationToken,
		"agent_name": agentName,
		"agent_info": sysInfo,
		"version":    sysInfo.Version,
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
	log.Println("Sent registration request to controller at:", controllerURL)
	if err != nil {
		return fmt.Errorf("error connecting to controller: %w", err)
	}
	defer resp.Body.Close()
	log.Println("Received response from controller with status:", resp.Status)
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
	if err := ch.TokenStore.StoreToken(regResp.AgentID, regResp.Token, controllerURL); err != nil {
		return fmt.Errorf("error storing permanent token: %w", err)
	}

	log.Printf("Successfully registered with controller as agent %s", regResp.AgentID)
	return nil
}

func NewAgentCommandHandler(logger *slog.Logger, TokenStore *store.TokenStore) *AgentCommandHandler {
	return &AgentCommandHandler{
		Logger:     logger,
		TokenStore: TokenStore,
	}
}
