package commands

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "lixies",
	Short: "GitOps CLI for managing container deployments",
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	joinCmd.Flags().String("token", "", "Registration token from the controller")
	joinCmd.Flags().String("controller", "", "Controller URL (e.g., http://lixy-controller.example.com:8080)")
	joinCmd.Flags().String("name", "", "Agent name (defaults to hostname)")
	// Add subcommands
	rootCmd.AddCommand(joinCmd)
}
