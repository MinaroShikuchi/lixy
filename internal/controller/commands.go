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
	gitopsReconciler  *services.GitOpsReconciler
	registryService   *services.RegistryService
	authService       *services.AuthService
	getSystemInfo     func() (*domain.SystemInfo, error)
}

func NewControllerCommandHandler(logger *slog.Logger, agentService *services.AgentService, deploymentService *services.DeploymentService, gitopsReconciler *services.GitOpsReconciler, registryService *services.RegistryService, authService *services.AuthService, GetSystemInfo func() (*domain.SystemInfo, error)) *ControllerCommandHandler {
	return &ControllerCommandHandler{
		logger:            logger,
		agentService:      agentService,
		deploymentService: deploymentService,
		gitopsReconciler:  gitopsReconciler,
		registryService:   registryService,
		authService:       authService,
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
	case "get-token":
		return ch.handleGetToken(cmd.Params)
	case "get-targets":
		return ch.handleGetTargets(cmd.Params)
	case "create-deployment":
		return ch.handleCreateDeployment(cmd.Params)
	case "update-deployment":
		return ch.handleUpdateDeployment(cmd.Params)
	case "delete-deployment":
		return ch.handleDeleteDeployment(cmd.Params)
	case "reconcile-environment-deployments":
		return ch.handleReconcileEnvironmentDeployments(cmd.Params)
	case "add-registry-credential":
		return ch.handleAddRegistryCredential(cmd.Params)
	case "list-registry-credentials":
		return ch.handleListRegistryCredentials(cmd.Params)
	case "delete-registry-credential":
		return ch.handleDeleteRegistryCredential(cmd.Params)
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
	token, err := ch.authService.GenerateRegistrationToken(time.Duration(options.Expiration))
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
	var params domain.GetDeploymentOptions
	if err := json.Unmarshal(options, &params); err != nil {
		return domain.Response{Success: false, Message: "Failed to unmarshal parameters"}
	}
	if params.Name == "" {
		return domain.Response{Success: false, Message: "Deployment name is required"}
	}
	deployment, err := ch.deploymentService.GetDeployment(params.Name)
	if err != nil {
		return domain.Response{Success: false, Message: "Failed to get deployment: " + err.Error()}
	}

	return domain.Response{Success: true, Data: deployment}
}

func (ch *ControllerCommandHandler) handleGetToken(params []byte) domain.Response {
	// Generate a temporary token valid for 10 minutes
	token, err := ch.authService.GenerateTemporaryToken("ui", 6*60*time.Minute)
	if err != nil {
		ch.logger.Error("Failed to generate token", "error", err)
		return domain.Response{Success: false, Message: "Failed to generate token: " + err.Error()}
	}
	return domain.Response{Success: true, Data: map[string]string{"token": token}}
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

func (ch *ControllerCommandHandler) handleReconcileEnvironmentDeployments(paramsRaw json.RawMessage) domain.Response {
	var params domain.ReconcileEnvironmentDeploymentsOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for reconcile-environment-deployments: %v", err)}
	}

	if params.Repository == "" || params.Environment == "" {
		return domain.Response{Success: false, Message: "Repository and environment are required"}
	}

	if ch.gitopsReconciler == nil {
		return domain.Response{Success: false, Message: "GitOps reconciler not available"}
	}

	ch.logger.Info("Starting environment deployment reconciliation",
		"repository", params.Repository,
		"environment", params.Environment)

	err := ch.gitopsReconciler.ReconcileEnvironmentDeployments(params.Repository, params.Environment)
	if err != nil {
		ch.logger.Error("Failed to reconcile environment deployments",
			"repository", params.Repository,
			"environment", params.Environment,
			"error", err)
		return domain.Response{Success: false, Message: "Failed to reconcile environment deployments: " + err.Error()}
	}

	ch.logger.Info("Successfully reconciled environment deployments",
		"repository", params.Repository,
		"environment", params.Environment)

	return domain.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully reconciled environment '%s' from repository '%s'", params.Environment, params.Repository),
		Data: map[string]interface{}{
			"repository":  params.Repository,
			"environment": params.Environment,
		},
	}
}

func (ch *ControllerCommandHandler) handleAddRegistryCredential(paramsRaw []byte) domain.Response {
	var params domain.AddRegistryCredentialOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for add-registry-credential: %v", err)}
	}

	if params.RegistryType == "" || params.Username == "" || params.Token == "" {
		return domain.Response{Success: false, Message: "Registry type, username, and token are required"}
	}

	if ch.registryService == nil {
		return domain.Response{Success: false, Message: "Registry service not available"}
	}

	err := ch.registryService.StoreCredential(params.RegistryType, params.Username, params.Token)
	if err != nil {
		ch.logger.Error("Failed to store registry credential",
			"registry", params.RegistryType,
			"error", err)
		return domain.Response{Success: false, Message: "Failed to store credential: " + err.Error()}
	}

	ch.logger.Info("Successfully stored registry credential",
		"registry", params.RegistryType,
		"username", params.Username)

	return domain.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully added credentials for registry: %s", params.RegistryType),
	}
}

func (ch *ControllerCommandHandler) handleListRegistryCredentials(paramsRaw []byte) domain.Response {
	if ch.registryService == nil {
		return domain.Response{Success: false, Message: "Registry service not available"}
	}

	credentials, err := ch.registryService.ListCredentials()
	if err != nil {
		ch.logger.Error("Failed to list registry credentials", "error", err)
		return domain.Response{Success: false, Message: "Failed to list credentials: " + err.Error()}
	}

	return domain.Response{
		Success: true,
		Data:    credentials,
	}
}

func (ch *ControllerCommandHandler) handleDeleteRegistryCredential(paramsRaw []byte) domain.Response {
	var params domain.DeleteRegistryCredentialOptions
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return domain.Response{Success: false, Message: fmt.Sprintf("Invalid parameters for delete-registry-credential: %v", err)}
	}

	if params.RegistryType == "" {
		return domain.Response{Success: false, Message: "Registry type is required"}
	}

	if ch.registryService == nil {
		return domain.Response{Success: false, Message: "Registry service not available"}
	}

	err := ch.registryService.DeleteCredential(params.RegistryType)
	if err != nil {
		ch.logger.Error("Failed to delete registry credential",
			"registry", params.RegistryType,
			"error", err)
		return domain.Response{Success: false, Message: "Failed to delete credential: " + err.Error()}
	}

	ch.logger.Info("Successfully deleted registry credential",
		"registry", params.RegistryType)

	return domain.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully deleted credentials for registry: %s", params.RegistryType),
	}
}
