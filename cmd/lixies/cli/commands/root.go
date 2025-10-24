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
	// Add subcommands
	registerCmd.Flags().String("token", "", "Registration token from the controller")
	registerCmd.Flags().String("controller", "", "Controller URL (e.g., http://lixy-controller.example.com:8080)")
	registerCmd.Flags().String("name", "", "Agent name (defaults to hostname)")

	unregisterCmd.Flags().String("controller", "", "Controller URL (e.g., http://lixy-controller.example.com:8080)")

	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)
}
