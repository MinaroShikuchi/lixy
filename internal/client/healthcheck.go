package client

import (
	"log"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/services"
	"github.com/MinaroShikuchi/lixy/internal/store"
)

// HealthChecker manages periodic health checks for all agents
type HealthChecker struct {
	agentStore    *store.AgentStore
	checkInterval time.Duration
	stopCh        chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(checkInterval time.Duration, agentStore *store.AgentStore) *HealthChecker {
	return &HealthChecker{
		agentStore:    agentStore,
		checkInterval: checkInterval,
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
	log.Println("Running health check for all agents...")

	// Get all agents via agentStore
	agents := c.agentStore.List()

	// If there are no agents, log a message and return
	if len(agents) == 0 {
		log.Println("No agents found; skipping health check")
		return
	}

	// Process each agent
	for _, agent := range agents {
		// Check agent's health
		healthy, message := services.CheckAgentHealth(agent, c.agentStore)

		// Log the result
		logLevel := "INFO"
		if !healthy {
			logLevel = "WARN"
		}
		log.Printf("[%s] Health check for agent %s (%s): %t - %s",
			logLevel, agent.Name, agent.IP, healthy, message)
	}

	log.Println("Health check completed for all agents")
}
