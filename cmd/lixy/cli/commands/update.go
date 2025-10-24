package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a resource",
	Long:  `Update an existing resource (deployment, target, etc.)`,
}

var updateDeploymentCmd = &cobra.Command{
	Use:          "deployment [NAME]",
	Short:        "Update an existing deployment",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate arguments
		if len(args) < 1 {
			return fmt.Errorf("deployment name is required")
		}

		deploymentName := args[0]

		// Get the compose file path flag
		composeFilePath, _ := cmd.Flags().GetString("compose-file")
		targetLXC, _ := cmd.Flags().GetString("target-lxc")

		// Validate required flags
		if composeFilePath == "" {
			return fmt.Errorf("--compose-file flag is required")
		}

		if targetLXC == "" {
			return fmt.Errorf("--target-lxc flag is required")
		}

		// Validate file exists
		if _, err := os.Stat(composeFilePath); errors.Is(err, os.ErrNotExist) {
			return errors.New("the specified compose file does not exist")
		}

		// TODO: Read the compose file
		// composeData, err := os.ReadFile(composeFilePath)
		// if err != nil {
		// 	return err
		// }

		fmt.Printf("Deployment %s updated successfully\n", deploymentName)
		return nil
	},
}

func init() {
	updateCmd.AddCommand(updateDeploymentCmd)

	// Configure flags
	updateDeploymentCmd.Flags().String("compose-file", "", "Path to updated Docker Compose file")
	updateDeploymentCmd.Flags().String("target-lxc", "", "Target LXC container ID")

	// Mark required flags
	updateDeploymentCmd.MarkFlagRequired("compose-file")
	updateDeploymentCmd.MarkFlagRequired("target-lxc")
}
