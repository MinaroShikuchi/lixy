package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/spf13/cobra"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile GitOps configurations",
	Long:  `Reconcile GitOps configurations from repository to create deployments`,
}

var reconcileEnvironmentCmd = &cobra.Command{
	Use:          "environment [ENVIRONMENT]",
	Short:        "Reconcile deployments from environment YAML files",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("environment name is required")
		}
		environment := args[0]

		repoName, _ := cmd.Flags().GetString("repository")
		if repoName == "" {
			repoName = "lixy-cd" // Default repository name
		}

		c := socket.NewControllerClient()

		params := map[string]string{
			"repository":  repoName,
			"environment": environment,
		}

		resp, err := c.SendCommand("reconcile-environment-deployments", params)
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("reconciliation failed: %s", resp.Message)
		}

		fmt.Printf("Successfully reconciled environment '%s' from repository '%s'\n", environment, repoName)
		if resp.Data != nil {
			if details, ok := resp.Data.(map[string]interface{}); ok {
				if count, exists := details["deployments_processed"]; exists {
					fmt.Printf("Deployments processed: %v\n", count)
				}
			}
		}

		return nil
	},
}

func init() {
	// Add flags
	reconcileEnvironmentCmd.Flags().StringP("repository", "r", "lixy-cd", "GitOps repository name")

	// Add subcommands
	reconcileCmd.AddCommand(reconcileEnvironmentCmd)
}
