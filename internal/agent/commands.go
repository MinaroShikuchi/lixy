// internal/agent/commands.go
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type AgentCommandHandler struct {
	logger        *slog.Logger
	getSystemInfo func() (*domain.SystemInfo, error)
	tokenService  *services.TokenService
}

func NewAgentCommandHandler(logger *slog.Logger, tokenService *services.TokenService, GetSystemInfo func() (*domain.SystemInfo, error)) *AgentCommandHandler {
	return &AgentCommandHandler{
		logger:        logger,
		getSystemInfo: GetSystemInfo,
		tokenService:  tokenService,
	}
}

func (ch *AgentCommandHandler) HandleCommand(cmd domain.Command) domain.Response {
	ch.logger.Info("Handling command", "action", cmd.Action)

	switch cmd.Action {
	case "register-agent":
		return ch.handleRegisterAgent(cmd.Params)
	case "unregister-agent":
		return ch.handleUnregisterAgent(cmd.Params)
	default:
		return domain.Response{Success: false, Message: "Unknown command"}
	}
}

// Command-specific handlers
func (ch *AgentCommandHandler) handleRegisterAgent(paramsRaw []byte) domain.Response {
	var params domain.RegisterOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for register-agent: %v", err)}
	}

	if err := ch.registerWithController(params.Controller, params.Token); err != nil {
		ch.logger.Error("Agent registration failed", "error", err)
		return domain.Response{Success: false, Message: "Registration failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: "Agent registered successfully"}
}

func (ch *AgentCommandHandler) handleUnregisterAgent(paramsRaw []byte) domain.Response {
	var params domain.UnregisterOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for unregister-agent: %v", err)}
	}

	if err := ch.unregisterWithController(); err != nil {
		ch.logger.Error("Agent unregistration failed", "error", err)
		return domain.Response{Success: false, Message: "Unregistration failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: "Agent unregistered successfully"}
}

// RegisterWithController registers this agent with the controller using the provided token
func (ch *AgentCommandHandler) registerWithController(controllerURL, registrationToken string) error {
	// Get system information
	sysInfo, err := ch.getSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}
	body := domain.RegistrationRequest{
		Token:    registrationToken,
		Hostname: sysInfo.Hostname,
	}
	reqBody, err := json.Marshal(body)

	if err != nil {
		return fmt.Errorf("error preparing registration request: %w", err)
	}

	// Send registration request
	resp, err := http.Post(
		fmt.Sprintf("%s/api/register-agent", controllerURL),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	ch.logger.Info("Sent registration request to controller", "controler-url", controllerURL)
	if err != nil {
		return fmt.Errorf("error connecting to controller: %w", err)
	}
	defer resp.Body.Close()
	ch.logger.Info("Received response from controller", "status", resp.Status)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}

	var registrationResponse domain.RegistrationResponse

	if err := json.NewDecoder(resp.Body).Decode(&registrationResponse); err != nil {
		return fmt.Errorf("error parsing registration response: %w", err)
	}
	if err := ch.tokenService.CreateToken(registrationResponse.Token, controllerURL); err != nil {
		return fmt.Errorf("error storing permanent token: %w", err)
	}

	req := domain.SaveAgentRequest{
		Version: sysInfo.Version,
		IP:      sysInfo.IP, // Using hostname as a placeholder for IP
		Port:    sysInfo.Port,
	}

	// Call controller securly endpoint to verify registration
	agentReqBody, err := json.Marshal(req)

	if err != nil {
		return fmt.Errorf("error preparing verification request: %w", err)
	}

	verifyReq, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/save-agent", controllerURL),
		bytes.NewBuffer(agentReqBody),
	)
	if err != nil {
		return fmt.Errorf("error creating verification request: %w", err)
	}
	verifyReq.Header.Set("Content-Type", "application/json")
	// Use the token returned by registration to authorize the verification call
	verifyReq.Header.Set("Authorization", "Bearer "+registrationResponse.Token)

	client := &http.Client{}
	verifyResp, err := client.Do(verifyReq)
	if err != nil {
		return fmt.Errorf("error connecting to controller for verification: %w", err)
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(verifyResp.Body)
		return fmt.Errorf("verification failed: %s", body)
	}

	var verResult domain.SaveAgentResponse

	if err := json.NewDecoder(verifyResp.Body).Decode(&verResult); err != nil {
		return fmt.Errorf("error parsing verification response: %w", err)
	}
	if !verResult.Success {
		return fmt.Errorf("controller verification reported failure: %s", verResult.Message)
	}

	agentName := verResult.Data["agent_name"]
	ch.logger.Info("Controller verification succeeded", "agent_name", agentName)

	return nil
}

// UnregisterWithController unregisters this agent with the controller
func (ch *AgentCommandHandler) unregisterWithController() error {
	// Get system information
	sysInfo, err := ch.getSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	// Load the stored token for this agent
	tokenData, err := ch.tokenService.GetToken()
	if err != nil {
		return fmt.Errorf("error retrieving stored token: %w", err)
	}

	body := domain.UnregistrationRequest{}
	reqBody, err := json.Marshal(body)

	if err != nil {
		return fmt.Errorf("error preparing unregistration request: %w", err)
	}
	unregisterReq, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/unregister-agent", tokenData.ControllerURL),
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("error creating unregistration request: %w", err)
	}

	unregisterReq.Header.Set("Content-Type", "application/json")
	// Use the token returned by registration to authorize the verification call
	unregisterReq.Header.Set("Authorization", "Bearer "+tokenData.Token)

	ch.logger.Info("Sent unregistration request to controller", "controller-url", tokenData.ControllerURL)

	client := &http.Client{}
	resp, err := client.Do(unregisterReq)
	if err != nil {
		return fmt.Errorf("error connecting to controller for verification: %w", err)
	}
	defer resp.Body.Close()
	ch.logger.Info("Received response from controller", "status", resp.Status)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}

	ch.logger.Info("Controller verification succeeded", "agent_id", sysInfo.Hostname)
	return nil

}
