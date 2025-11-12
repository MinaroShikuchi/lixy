// internal/agent/commands.go
package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type AgentCommandHandler struct {
	logger              *slog.Logger
	registrationService *services.AgentRegistrationService
}

func NewAgentCommandHandler(logger *slog.Logger, registrationService *services.AgentRegistrationService) *AgentCommandHandler {
	return &AgentCommandHandler{
		logger:              logger,
		registrationService: registrationService,
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

	if err := ch.registrationService.RegisterWithController(params.Controller, params.Token); err != nil {
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

	if err := ch.registrationService.UnregisterWithController(); err != nil {
		ch.logger.Error("Agent unregistration failed", "error", err)
		return domain.Response{Success: false, Message: "Unregistration failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: "Agent unregistered successfully"}
}
