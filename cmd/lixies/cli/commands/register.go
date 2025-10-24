package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/MinaroShikuchi/lixy/internal/domain"

	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:           "register",
	Short:         "Register wuith lixy controller as a worker agent",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		controller, _ := cmd.Flags().GetString("controller")

		if token == "" {
			return fmt.Errorf("registration token is required (--token)")
		}

		if controller == "" {
			return fmt.Errorf("controller URL is required (--controller)")
		}

		fmt.Printf("Registering with controller at %s...\n", controller)
		err := registerWithController(controller, token)
		if err != nil {
			return err
		}
		fmt.Println("Successfully registered! This agent will now accept commands from the controller.")
		return nil
	},
}

// Register with controller
func registerWithController(controller string, token string) error {
	c := socket.NewAgentController()

	params := domain.RegisterOptions{
		Token:      token,
		Controller: controller,
	}

	resp, err := c.SendCommand("register-agent", params)

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Message)
	}

	return nil
}
