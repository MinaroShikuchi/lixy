// internal/controller/commands.go
package controller

import (
	"fmt"
	"log/slog"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

type ControllerCommandHandler struct {
	Logger     *slog.Logger
	AgentStore *store.AgentStore
}

func (ch *ControllerCommandHandler) HandleCommand(cmd domain.Command) domain.Response {
	ch.Logger.Info("Handling command", "action", cmd.Action)
	switch cmd.Action {
	case "get-deployments":
		return ch.handleGetDeployments(cmd)
	case "get-deployment":
		return ch.handleGetDeployment(cmd)
	case "get-targets":
		return ch.handleGetTargets(cmd)
	case "create-deployment":
		return ch.handleCreateDeployment(cmd)
	case "delete-deployment":
		return ch.handleDeleteDeployment(cmd)
	default:
		return domain.Response{Success: false, Message: "Unknown command"}
	}
}

// Command-specific handlers
func (ch *ControllerCommandHandler) handleGetDeployments(cmd domain.Command) domain.Response {
	// Handle list all deployments
	deployments := []map[string]string{
		{"name": "app1", "targetLXC": "101", "status": "running"},
		{"name": "app2", "targetLXC": "102", "status": "stopped"},
	}

	return domain.Response{Success: true, Data: deployments}
}
func (ch *ControllerCommandHandler) handleGetDeployment(cmd domain.Command) domain.Response {
	// Handle get specific deployment
	name := cmd.Params["name"]
	// In a real implementation, look up the deployment by name
	deployment := map[string]string{
		"name":      name,
		"targetLXC": "101",
		"status":    "running",
		"image":     "my-app:latest",
		"created":   "2023-01-01 12:00:00",
	}
	return domain.Response{Success: true, Data: deployment}
}

func (ch *ControllerCommandHandler) handleGetTargets(cmd domain.Command) domain.Response {
	// Get agents using the agent store
	agents := ch.AgentStore.ListAgents()
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

func (ch *ControllerCommandHandler) handleCreateDeployment(cmd domain.Command) domain.Response {
	// Extract parameters
	name := cmd.Params["name"]
	targetLXC := cmd.Params["target_lxc"]
	composeYAML := cmd.Params["compose_yaml"]

	// Validate parameters
	if name == "" || targetLXC == "" || composeYAML == "" {
		return domain.Response{Success: false, Message: "Missing required parameters"}
	}

	// // Validate compose file format
	// if err := services.ValidateDeployment(targetLXC, composeYAML); err != nil {
	// 	response = domain.Response{Success: false, Message: "Invalid compose file: " + err.Error()}
	// 	break
	// }

	err := services.DeployToTarget(name, targetLXC, composeYAML, ch.AgentStore)
	if err != nil {
		return domain.Response{Success: false, Message: "Deployment failed: " + err.Error()}
	}
	return domain.Response{Success: true, Message: fmt.Sprintf("Deployment %s to target %s initiated", name, targetLXC)}
}

func (ch *ControllerCommandHandler) handleDeleteDeployment(cmd domain.Command) domain.Response {
	// Handle delete deployment
	name := cmd.Params["name"]
	targetLXC := cmd.Params["target_lxc"]

	// Validate parameters
	if name == "" || targetLXC == "" {
		return domain.Response{Success: false, Message: "Missing required parameters"}
	}
	// In a real implementation, delete the deployment by name
	err := services.DeleteDeployment(name, targetLXC, ch.AgentStore)
	if err != nil {
		ch.Logger.Error("Failed to delete deployment", "name", name, "error", err)
		return domain.Response{Success: false, Message: "Failed to delete deployment: " + err.Error()}
	}
	ch.Logger.Info("Deleted deployment", "name", name)
	return domain.Response{Success: true, Message: fmt.Sprintf("Deployment %s deleted successfully", name)}
}

// Other dependencies...
func NewControllerCommandHandler(logger *slog.Logger, agentStore *store.AgentStore) *ControllerCommandHandler {
	return &ControllerCommandHandler{
		Logger:     logger,
		AgentStore: agentStore,
	}
}
