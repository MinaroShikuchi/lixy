package commands

import (
	"fmt"
	"os"

	"github.com/MinaroShikuchi/lixy/pkg/client"

	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a lixy controller as a worker agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		controller, _ := cmd.Flags().GetString("controller")
		name, _ := cmd.Flags().GetString("name")

		c := client.LyxiClient("/tmp/lixies.sock")

		if token == "" {
			return fmt.Errorf("registration token is required (--token)")
		}

		if controller == "" {
			return fmt.Errorf("controller URL is required (--controller)")
		}

		if name == "" {
			// Use hostname as default
			hostname, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("failed to get hostname: %w", err)
			}
			name = hostname
		}

		fmt.Printf("Registering with controller at %s...\n", controller)
		err := c.RegisterWithController(controller, token, name)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		fmt.Println("Successfully registered! This agent will now accept commands from the controller.")
		return nil
	},
}
