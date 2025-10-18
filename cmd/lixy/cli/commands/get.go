// cmd/cli/commands/get.go
package commands

import (
	"github.com/spf13/cobra"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Display one or many resources",
	Long:  `Displays resource information (deployments, targets, etc.)`,
}

func init() {
	// Add subcommands to get
	getCmd.AddCommand(deploymentsCmd)
	getCmd.AddCommand(targetsCmd)

	// Add other get subcommands
	// getCmd.AddCommand(logsCmd)
}
