package client

import (
	"log/slog"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

// HealthChecker manages periodic health checks for all agents
type HealthChecker struct {
	logger        *slog.Logger
	agentStore    *store.AgentStore
	checkInterval time.Duration
	stopCh        chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(checkInterval time.Duration, agentStore *store.AgentStore, logger *slog.Logger) *HealthChecker {
	return &HealthChecker{
		agentStore:    agentStore,
		checkInterval: checkInterval,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the periodic health checking
func (c *HealthChecker) Start() {
	ticker := time.NewTicker(c.checkInterval)
	go func() {
		// Run an immediate check when starting
		c.checkAllAgents()

		for {
			select {
			case <-ticker.C:
				c.checkAllAgents()
			case <-c.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop ends the periodic health checking
func (c *HealthChecker) Stop() {
	close(c.stopCh)
}

// checkAllAgents verifies the health of all registered agents
func (c *HealthChecker) checkAllAgents() {
	c.logger.Info("Running health check for all agents")

	// Get all agents via agentStore
	agents := c.agentStore.List()

	// If there are no agents, log a message and return
	if len(agents) == 0 {
		c.logger.Info("No agents found; skipping health check")
		return
	}

	// Process each agent
	for _, agent := range agents {
		// Check agent's health
		healthy, message := services.CheckAgentHealth(agent, c.agentStore)

		// Log the result
		if !healthy {
			c.logger.Warn("Agent health check failed", "agent", agent.Name, "ip", agent.IP, "message", message)
		} else {
			c.logger.Info("Agent is healthy", "agent", agent.Name, "ip", agent.IP)
		}
	}

	c.logger.Info("Health check completed for all agents")
}
