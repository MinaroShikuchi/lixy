package commands

import "github.com/spf13/cobra"

// These variables will be set by the linker at build time
var (
	// Version is the application version, usually set to a git tag (e.g., v1.2.3)
	Version = "dev"

	// Commit is the git commit hash
	Commit = "none"

	// Date is the build date
	Date = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "lixies",
	Short:   "GitOps CLI for managing container deployments",
	Version: Version,
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
