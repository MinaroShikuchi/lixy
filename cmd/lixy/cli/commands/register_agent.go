// cmd/lixy-cli/commands/register_agent.go
package commands

import (
	"fmt"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/spf13/cobra"
)

// registerAgentCmd represents the register-agent command
var registerAgentCmd = &cobra.Command{
	Use:          "register-agent",
	Short:        "Generate a registration token for a new lixies agent",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the expiration flag
		expiration, _ := cmd.Flags().GetDuration("expiration")

		c := socket.NewControllerClient()

		// debug
		fmt.Printf("Expiration: %s\n", expiration.String())
		fmt.Printf("Expiration: %d\n", int64(expiration.Seconds()))

		params := domain.RegisterAgentOptions{
			Expiration: int(expiration.Seconds()),
		}

		resp, err := c.SendCommand("register-agent", params)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Message)
		}
		// Extract token and controller URL from response
		respData, ok := resp.Data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid response format")
		}

		fmt.Printf("Run this command on your lixies agent to register (valid for %s):\r", expiration)
		fmt.Printf("lixies register --token %s --controller %s\n", respData["token"], respData["controller"])
		return nil
	},
}

func init() {
	// Initialize the command
	registerAgentCmd.Flags().DurationP("expiration", "e", 10*time.Minute, "Expiration duration for the registration token")

}
