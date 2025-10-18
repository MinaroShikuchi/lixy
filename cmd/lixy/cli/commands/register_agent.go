// cmd/lixy-cli/commands/register_agent.go
package commands

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/MinaroShikuchi/lixy/internal/auth"
)

var (
	// Define the controller URL at package level
	controllerURL string
)

// registerAgentCmd represents the register-agent command
var registerAgentCmd = &cobra.Command{
	Use:   "register-agent",
	Short: "Generate a registration token for a new lixies agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		expiration, _ := cmd.Flags().GetDuration("expiration")

		// Initialize authentication system
		if err := auth.InitializeAuth(); err != nil {
			log.Fatalf("Failed to initialize authentication: %v", err)
		}

		// Call controller API to generate token
		token, err := auth.GenerateRegistrationToken(expiration)
		if err != nil {
			return err
		}

		fmt.Printf("Registration Token (valid for %s):\n\n%s\n\n", expiration, token)
		fmt.Println("Run this command on your lixies agent to register:")
		fmt.Printf("lixies join --token %s --controller %s\n", token, controllerURL)

		return nil
	},
}

func init() {
	// Initialize the command
	registerAgentCmd.Flags().DurationP("expiration", "e", time.Hour, "Token expiration time")

	// Add the controller URL flag and bind it to the variable
	registerAgentCmd.Flags().StringVar(&controllerURL, "controller", "http://localhost:8080", "URL of the lixy controller")

	// Add the command to root
}
