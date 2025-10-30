// internal/controller/commands.go
package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

type ControllerCommandHandler struct {
	logger            *slog.Logger
	agentService      *services.AgentService
	deploymentService *services.DeploymentService
	getSystemInfo     func() (*domain.SystemInfo, error)
}

func NewControllerCommandHandler(logger *slog.Logger, agentService *services.AgentService, deploymentService *services.DeploymentService, GetSystemInfo func() (*domain.SystemInfo, error)) *ControllerCommandHandler {
	return &ControllerCommandHandler{
		logger:            logger,
		agentService:      agentService,
		deploymentService: deploymentService,
		getSystemInfo:     GetSystemInfo,
	}
}

func (ch *ControllerCommandHandler) HandleCommand(cmd domain.Command) domain.Response {
	ch.logger.Info("Handling command", "action", cmd.Action)

	switch cmd.Action {
	case "register-agent":
		return ch.handleRegisterAgent(cmd.Params)
	case "get-deployments":
		return ch.handleGetDeployments(cmd.Params)
	case "get-deployment":
		return ch.handleGetDeployment(cmd.Params)
	case "get-targets":
		return ch.handleGetTargets(cmd.Params)
	case "create-deployment":
		return ch.handleCreateDeployment(cmd.Params)
	case "update-deployment":
		return ch.handleUpdateDeployment(cmd.Params)
	case "delete-deployment":
		return ch.handleDeleteDeployment(cmd.Params)
	default:
		return domain.Response{Success: false, Message: "Unknown command"}
	}
}

func (ch *ControllerCommandHandler) handleRegisterAgent(params []byte) domain.Response {
	var options domain.RegisterAgentOptions
	if err := json.Unmarshal(params, &options); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for register-agent: %v", err)}
	}

	// Generate a registration token
	token, err := services.GenerateRegistrationToken(time.Duration(options.Expiration))
	if err != nil {
		ch.logger.Error("Failed to generate registration token", "error", err)
		return domain.Response{Success: false, Message: "Failed to generate registration token: " + err.Error()}
	}
	sysInfo, err := ch.getSystemInfo()
	if err != nil {
		ch.logger.Error("Failed to get system info", "error", err)
		return domain.Response{Success: false, Message: "Failed to get system info: " + err.Error()}
	}
	controllerUrl := fmt.Sprintf("http://%s:%d", sysInfo.IP, sysInfo.Port)
	return domain.Response{Success: true, Data: map[string]string{"token": token, "controller": controllerUrl}}
}

func (ch *ControllerCommandHandler) handleGetDeployments(params []byte) domain.Response {
	// Handle list all deployments
	deployments, err := ch.deploymentService.ListAllDeployments("")
	if err != nil {
		return domain.Response{Success: false, Message: "Failed to list deployments: " + err.Error()}
	}
	return domain.Response{Success: true, Data: deployments}
}
func (ch *ControllerCommandHandler) handleGetDeployment(options []byte) domain.Response {
	params := make(map[string]string)
	err := json.Unmarshal(options, &params)
	if err != nil {
		return domain.Response{Success: false, Message: "Failed to unmarshal parameters"}
	}
	deployments, err := ch.deploymentService.ListAllDeployments("")
	if err != nil {
		return domain.Response{Success: false, Message: "Failed to list deployments: " + err.Error()}
	}

	return domain.Response{Success: true, Data: deployments}
}

func (ch *ControllerCommandHandler) handleGetTargets(params []byte) domain.Response {
	// Get agents using the agent store
	agents := ch.agentService.ListAgents()
	// Convert agents to targets format
	targets := make([]map[string]string, 0, len(agents))
	for _, agent := range agents {
		target := map[string]string{
			"id":     agent.ID,
			"name":   agent.Name,
			"status": agent.Status,
			"ip":     agent.IP,
		}

		// Add selected metadata fields if needed
		for key, value := range agent.Metadata {
			// Only include specific metadata fields you want in the response
			if key == "version" || key == "os" || key == "arch" {
				target[key] = value
			}
		}

		targets = append(targets, target)
	}
	return domain.Response{Success: true, Data: targets}
}

func (ch *ControllerCommandHandler) handleCreateDeployment(paramsRaw []byte) domain.Response {
	var params domain.CreateDeploymentOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for create-deployment: %v", err)}
	}

	// // Validate compose file format
	// if err := services.ValidateDeployment(targetLXC, composeYAML); err != nil {
	// 	response = domain.Response{Success: false, Message: "Invalid compose file: " + err.Error()}
	// 	break
	// }
	err := ch.deploymentService.CreateDeployment(params.Name, params.TargetLXC, params.ComposeYAML)
	if err != nil {
		return domain.Response{Success: false, Message: "Deployment failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: fmt.Sprintf("Deployment %s to target %s created", params.Name, params.TargetLXC)}
}

func (ch *ControllerCommandHandler) handleUpdateDeployment(paramsRaw []byte) domain.Response {
	var params domain.CreateDeploymentOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for update-deployment: %v", err)}
	}

	err := ch.deploymentService.UpdateDeployment(params.Name, params.ComposeYAML)
	if err != nil {
		return domain.Response{Success: false, Message: "Update failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: fmt.Sprintf("Deployment %s to target %s updated", params.Name, params.TargetLXC)}
}

func (ch *ControllerCommandHandler) handleDeleteDeployment(paramsRaw []byte) domain.Response {
	var params domain.DeleteDeploymentOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for delete-deployment: %v", err)}
	}
	name := params.Name
	// In a real implementation, delete the deployment by name
	err := ch.deploymentService.DeleteDeployment(name)
	if err != nil {
		ch.logger.Error("Failed to delete deployment", "name", name, "error", err)
		return domain.Response{Success: false, Message: "Failed to delete deployment: " + err.Error()}
	}
	ch.logger.Info("Deleted deployment", "name", name)
	return domain.Response{Success: true, Message: fmt.Sprintf("Deployment %s deleted successfully", name)}
}
