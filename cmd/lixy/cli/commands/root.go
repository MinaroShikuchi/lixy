// cmd/cli/commands/root.go
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
	Use:     "lixy",
	Short:   "GitOps CLI for managing container deployments",
	Version: Version,
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
	rootCmd.AddCommand(reconcileCmd)
}
