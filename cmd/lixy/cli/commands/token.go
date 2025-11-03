package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:          "token",
	Short:        "Generate a temporary token",
	Long:         `Commands to generate`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := socket.NewControllerClient()

		resp, err := c.SendCommand("get-token", nil)
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("error: %s", resp.Message)
		}

		fmt.Printf("Generated Token availaible for 10min:\n%s\n", resp.Data.(map[string]interface{})["token"])
		return nil

	},
}
