package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a resource",
	Long:  `Delete an existing resource (deployment, target, etc.)`,
}

var deleteDeploymentCmd = &cobra.Command{
	Use:          "deployment [NAME]",
	Short:        "Delete an existing deployment",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := socket.NewControllerClient()

		if len(args) < 1 {
			return nil // Or return an error indicating that the name is required
		}
		deploymentName := args[0]
		targetLXC, _ := cmd.Flags().GetString("target-lxc")

		params := map[string]string{"name": deploymentName}
		if targetLXC != "" {
			params["target_lxc"] = targetLXC
		}

		resp, err := c.SendCommand("delete-deployment", params)
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("error: %s", resp.Message)
		}

		fmt.Printf("Deployment %s deleted successfully\n", deploymentName)

		return nil
	},
}

func init() {
	deleteCmd.AddCommand(deleteDeploymentCmd)
	// Add flags if needed
	deleteDeploymentCmd.Flags().Bool("force", false, "Force deletion without confirmation")
	deleteDeploymentCmd.Flags().String("target-lxc", "", "Specify the target LXC for the deployment")
}
