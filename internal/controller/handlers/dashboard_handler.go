package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/services"
)

// DashboardHandlers handles dashboard-related HTTP requests
type DashboardHandlers struct {
	logger            *slog.Logger
	agentService      *services.AgentService
	deploymentService *services.DeploymentService
	gitService        *services.GitService
}

// NewDashboardHandlers creates a new dashboard handlers instance
func NewDashboardHandlers(
	logger *slog.Logger,
	agentService *services.AgentService,
	deploymentService *services.DeploymentService,
	gitService *services.GitService,
) *DashboardHandlers {
	return &DashboardHandlers{
		logger:            logger,
		agentService:      agentService,
		deploymentService: deploymentService,
		gitService:        gitService,
	}
}

// DashboardOverview represents the main dashboard overview data
type DashboardOverview struct {
	Agents struct {
		Total      int `json:"total"`
		Online     int `json:"online"`
		Offline    int `json:"offline"`
		Registered int `json:"registered"`
	} `json:"agents"`
	Deployments struct {
		Total   int `json:"total"`
		Running int `json:"running"`
		Stopped int `json:"stopped"`
		Pending int `json:"pending"`
		Failed  int `json:"failed"`
	} `json:"deployments"`
	Repositories struct {
		Total int `json:"total"`
	} `json:"repositories"`
	System struct {
		Uptime   string    `json:"uptime"`
		LastSync time.Time `json:"last_sync,omitempty"`
		Version  string    `json:"version"`
	} `json:"system"`
}

// GetOverview returns dashboard overview statistics
func (h *DashboardHandlers) GetOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	overview := DashboardOverview{}

	// Get agent statistics
	agents := h.agentService.ListAgents()
	overview.Agents.Total = len(agents)
	overview.Agents.Registered = len(agents)

	// Count online/offline agents (simple health check)
	for _, agent := range agents {
		// You could enhance this with actual health checks
		if agent.IP != "" {
			overview.Agents.Online++
		} else {
			overview.Agents.Offline++
		}
	}

	// Get deployment statistics
	deployments, err := h.deploymentService.ListAllDeployments("")
	if err != nil {
		h.logger.Error("Failed to list deployments", "error", err)
	} else {
		overview.Deployments.Total = len(deployments)
		for _, dep := range deployments {
			switch dep.Status {
			case "running":
				overview.Deployments.Running++
			case "stopped":
				overview.Deployments.Stopped++
			case "pending":
				overview.Deployments.Pending++
			case "failed":
				overview.Deployments.Failed++
			}
		}
	}

	// Get repository count
	repos := h.gitService.ListRepositories()
	overview.Repositories.Total = len(repos)

	// System info
	overview.System.Version = "0.1.0" // You can get this from version.go
	overview.System.Uptime = "N/A"    // Implement uptime tracking if needed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}

// DashboardStats represents detailed statistics
type DashboardStats struct {
	DeploymentsByAgent  map[string]int     `json:"deployments_by_agent"`
	DeploymentsByStatus map[string]int     `json:"deployments_by_status"`
	RecentDeployments   []RecentDeployment `json:"recent_deployments"`
	AgentDetails        []AgentDetail      `json:"agent_details"`
}

// RecentDeployment represents a recent deployment
type RecentDeployment struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Target    string    `json:"target"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// AgentDetail represents agent details for dashboard
type AgentDetail struct {
	Hostname        string `json:"hostname"`
	IP              string `json:"ip"`
	Port            int    `json:"port"`
	Version         string `json:"version"`
	DeploymentCount int    `json:"deployment_count"`
}

// GetStats returns detailed dashboard statistics
func (h *DashboardHandlers) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := DashboardStats{
		DeploymentsByAgent:  make(map[string]int),
		DeploymentsByStatus: make(map[string]int),
		RecentDeployments:   make([]RecentDeployment, 0),
		AgentDetails:        make([]AgentDetail, 0),
	}

	// Get all deployments
	deployments, err := h.deploymentService.ListAllDeployments("")
	if err != nil {
		h.logger.Error("Failed to list deployments", "error", err)
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	// Count deployments by agent and status
	for _, dep := range deployments {
		stats.DeploymentsByAgent[dep.TargetLXC]++
		stats.DeploymentsByStatus[dep.Status]++
	}

	// Get recent deployments (last 10)
	maxRecent := 10
	for i, dep := range deployments {
		if i >= maxRecent {
			break
		}
		stats.RecentDeployments = append(stats.RecentDeployments, RecentDeployment{
			Name:   dep.Name,
			Status: dep.Status,
			Target: dep.TargetLXC,
		})
	}

	// Get agent details
	agents := h.agentService.ListAgents()
	for _, agent := range agents {
		version := "N/A"
		if v, ok := agent.Metadata["version"]; ok {
			version = v
		}

		detail := AgentDetail{
			Hostname:        agent.Name,
			IP:              agent.IP,
			Port:            agent.Port,
			Version:         version,
			DeploymentCount: stats.DeploymentsByAgent[agent.Name],
		}
		stats.AgentDetails = append(stats.AgentDetails, detail)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ActivityEvent represents a system activity event
type ActivityEvent struct {
	Type      string    `json:"type"`     // "deployment", "agent", "gitops"
	Action    string    `json:"action"`   // "created", "updated", "deleted", "registered"
	Resource  string    `json:"resource"` // Resource name
	Target    string    `json:"target"`   // Agent or environment
	Status    string    `json:"status"`   // "success", "failed", "pending"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// GetRecentActivity returns recent system activity
func (h *DashboardHandlers) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This is a simplified version - you would enhance this with actual event tracking
	activities := make([]ActivityEvent, 0)

	// Get recent deployments as activities
	deployments, err := h.deploymentService.ListAllDeployments("")
	if err == nil {
		maxEvents := 20
		for i, dep := range deployments {
			if i >= maxEvents {
				break
			}

			action := "updated"
			if dep.Status == "pending" {
				action = "created"
			}

			activities = append(activities, ActivityEvent{
				Type:      "deployment",
				Action:    action,
				Resource:  dep.Name,
				Target:    dep.TargetLXC,
				Status:    dep.Status,
				Message:   "Deployment " + action,
				Timestamp: time.Now(), // You'd track this properly
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"activities": activities,
		"count":      len(activities),
	})
}
