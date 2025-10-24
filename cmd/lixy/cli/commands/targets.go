// cmd/cli/commands/targets.go
package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/spf13/cobra"
)

// Define the target data structure
type Target struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	// Metadata map[string]string `json:"metadata,omitempty"`
}

// targetsCmd represents the targets command
var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "List all registered target LXC containers",
	Long: `Display information about all registered target LXC containers 
that are managed by the GitOps agent.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get target data from the socket
		c := socket.NewControllerClient()

		targets, err := c.SendCommand("get-targets", nil)
		if err != nil {
			return err
		}

		if !targets.Success {
			return fmt.Errorf("error: %s", targets.Message)
		}

		items, ok := targets.Data.([]interface{})
		if !ok {
			return fmt.Errorf("invalid response format")
		}

		// Display targets in a formatted table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tIP ADDRESS\t")
		fmt.Fprintln(w, "----\t----\t------\t----------\t")

		for _, item := range items {
			target, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
				target["id"],
				target["name"],
				target["status"],
				target["ip"])
		}
		w.Flush()

		return nil
	},
}
