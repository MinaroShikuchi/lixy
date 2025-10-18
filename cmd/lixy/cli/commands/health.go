// cmd/cli/commands/health.go
package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// healthCmd represents the health check command
var healthCmd = &cobra.Command{
	Use:          "health TARGET_LXC",
	Short:        "Check the health status of a lixies agent",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1), // Require exactly one argument
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]

		fmt.Printf("Checking health of lixies agent on LXC %s...\n", target)

		status, err := checkTargetHealth(target)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

		if status.Healthy {
			fmt.Printf("✅ %s: Healthy", target)
			if status.Message != "" {
				fmt.Printf(" - %s", status.Message)
			}
			fmt.Println()
			return nil
		} else {
			fmt.Printf("❌ %s: Unhealthy", target)
			if status.Message != "" {
				fmt.Printf(" - %s", status.Message)
			}
			fmt.Println()
			return fmt.Errorf("agent is unhealthy")
		}
	},
}

// Define the health check response structure
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// Check the health of a single target
func checkTargetHealth(targetLXC string) (*HealthStatus, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("http://%s:8765/healthz", targetLXC)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HealthStatus{
			Healthy: false,
			Message: fmt.Sprintf("returned status %d", resp.StatusCode),
		}, nil
	}

	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return &HealthStatus{
			Healthy: false,
			Message: "invalid response format",
		}, nil
	}

	return &status, nil
}
