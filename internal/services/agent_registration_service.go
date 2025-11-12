package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// AgentRegistrationService handles agent registration and unregistration with the controller
type AgentRegistrationService struct {
	logger        *slog.Logger
	tokenService  *TokenService
	getSystemInfo func() (*domain.SystemInfo, error)
	httpClient    *http.Client
}

// NewAgentRegistrationService creates a new agent registration service
func NewAgentRegistrationService(
	logger *slog.Logger,
	tokenService *TokenService,
	getSystemInfo func() (*domain.SystemInfo, error),
) *AgentRegistrationService {
	return &AgentRegistrationService{
		logger:        logger,
		tokenService:  tokenService,
		getSystemInfo: getSystemInfo,
		httpClient:    &http.Client{},
	}
}

// RegisterWithController registers this agent with the controller using the provided token
func (s *AgentRegistrationService) RegisterWithController(controllerURL, registrationToken string) error {
	// Get system information
	sysInfo, err := s.getSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	// Prepare registration request
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
	s.logger.Info("Sent registration request to controller", "controller-url", controllerURL)
	if err != nil {
		return fmt.Errorf("error connecting to controller: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Info("Received response from controller", "status", resp.Status)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}

	// Parse registration response
	var registrationResponse domain.RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&registrationResponse); err != nil {
		return fmt.Errorf("error parsing registration response: %w", err)
	}

	// Store the permanent token
	if err := s.tokenService.CreateToken(registrationResponse.Token, controllerURL); err != nil {
		return fmt.Errorf("error storing permanent token: %w", err)
	}

	// Verify registration by saving agent details
	if err := s.saveAgentDetails(controllerURL, registrationResponse.Token, sysInfo); err != nil {
		return fmt.Errorf("error verifying registration: %w", err)
	}

	return nil
}

// saveAgentDetails sends agent details to the controller for verification
func (s *AgentRegistrationService) saveAgentDetails(controllerURL, token string, sysInfo *domain.SystemInfo) error {
	req := domain.SaveAgentRequest{
		Version: sysInfo.Version,
		IP:      sysInfo.IP,
		Port:    sysInfo.Port,
	}

	// Prepare request body
	agentReqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error preparing verification request: %w", err)
	}

	// Create HTTP request
	verifyReq, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/save-agent", controllerURL),
		bytes.NewBuffer(agentReqBody),
	)
	if err != nil {
		return fmt.Errorf("error creating verification request: %w", err)
	}

	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("Authorization", "Bearer "+token)

	// Send request
	verifyResp, err := s.httpClient.Do(verifyReq)
	if err != nil {
		return fmt.Errorf("error connecting to controller for verification: %w", err)
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(verifyResp.Body)
		return fmt.Errorf("verification failed: %s", body)
	}

	// Parse verification response
	var verResult domain.SaveAgentResponse
	if err := json.NewDecoder(verifyResp.Body).Decode(&verResult); err != nil {
		return fmt.Errorf("error parsing verification response: %w", err)
	}

	if !verResult.Success {
		return fmt.Errorf("controller verification reported failure: %s", verResult.Message)
	}

	agentName := verResult.Data["agent_name"]
	s.logger.Info("Controller verification succeeded", "agent_name", agentName)

	return nil
}

// UnregisterWithController unregisters this agent with the controller
func (s *AgentRegistrationService) UnregisterWithController() error {
	// Get system information
	sysInfo, err := s.getSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	// Load the stored token for this agent
	tokenData, err := s.tokenService.GetToken()
	if err != nil {
		return fmt.Errorf("error retrieving stored token: %w", err)
	}

	// Prepare unregistration request
	body := domain.UnregistrationRequest{}
	reqBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error preparing unregistration request: %w", err)
	}

	// Create HTTP request
	unregisterReq, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/unregister-agent", tokenData.ControllerURL),
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("error creating unregistration request: %w", err)
	}

	unregisterReq.Header.Set("Content-Type", "application/json")
	unregisterReq.Header.Set("Authorization", "Bearer "+tokenData.Token)

	s.logger.Info("Sent unregistration request to controller", "controller-url", tokenData.ControllerURL)

	// Send request
	resp, err := s.httpClient.Do(unregisterReq)
	if err != nil {
		return fmt.Errorf("error connecting to controller for unregistration: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Info("Received response from controller", "status", resp.Status)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", body)
	}

	s.logger.Info("Controller unregistration succeeded", "agent_id", sysInfo.Hostname)
	return nil
}
