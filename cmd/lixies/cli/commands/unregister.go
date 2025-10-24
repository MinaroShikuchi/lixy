package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/MinaroShikuchi/lixy/internal/domain"

	"github.com/spf13/cobra"
)

var unregisterCmd = &cobra.Command{
	Use:           "unregister",
	Short:         "unregister from lixy controller",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := unexpectedregisterWithController()
		if err != nil {
			return err
		}
		fmt.Println("Successfully unregistered! This agent will no longer accept commands from the controller.")
		return nil
	},
}

// Register with controller
func unexpectedregisterWithController() error {
	c := socket.NewAgentController()

	params := domain.UnregisterOptions{}

	resp, err := c.SendCommand("unregister-agent", params)

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Message)
	}

	return nil
}
