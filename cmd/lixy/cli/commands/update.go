package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
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

		name := args[0]

		// Get the compose file path flag
		composeFilePath, _ := cmd.Flags().GetString("compose-file")

		// Validate required flags
		if composeFilePath == "" {
			return fmt.Errorf("--compose-file flag is required")
		}

		// Validate file exists
		if _, err := os.Stat(composeFilePath); errors.Is(err, os.ErrNotExist) {
			return errors.New("the specified compose file does not exist")
		}

		// Read the compose file
		composeData, err := os.ReadFile(composeFilePath)
		if err != nil {
			return err
		}
		c := socket.NewControllerClient()
		// Send create-deployment command to the agent
		err = c.UpdateDeployment(name, composeData)
		if err != nil {
			return fmt.Errorf("failed to update deployment: %v", err)
		}

		fmt.Printf("Deployment %s updated successfully\n", name)
		return nil
	},
}

func init() {
	updateCmd.AddCommand(updateDeploymentCmd)

	// Configure flags
	updateDeploymentCmd.Flags().String("compose-file", "", "Path to updated Docker Compose file")

	// Mark required flags
	updateDeploymentCmd.MarkFlagRequired("compose-file")
}
