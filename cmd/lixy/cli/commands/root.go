// cmd/cli/commands/root.go
package commands

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "lixy",
	Short: "GitOps CLI for managing container deployments",
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(registerAgentCmd)
}
