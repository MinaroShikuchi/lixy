package commands

import (
	"errors"
	"os"

	"github.com/MinaroShikuchi/lixy/pkg/client"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a resource",
	Long:  `Create a new resource (deployment, target, etc.)`,
}

var createDeploymentCmd = &cobra.Command{
	Use:   "deployment [NAME]",
	Short: "Create a new deployment",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		targetLXC, _ := cmd.Flags().GetString("target-lxc")
		composeFile, _ := cmd.Flags().GetString("compose-file")

		// Validate required flags
		if targetLXC == "" {
			return errors.New("the --target-lxc flag is required")
		}
		if composeFile == "" {
			return errors.New("the --compose-file flag is required")
		}

		// Validate file exists
		if _, err := os.Stat(composeFile); errors.Is(err, os.ErrNotExist) {
			return errors.New("the specified compose file does not exist")
		}

		// Read the compose file
		composeData, err := os.ReadFile(composeFile)
		if err != nil {
			return err
		}

		// Send create-deployment command to the agent
		createDeployment(name, targetLXC, composeData)

		return nil
	},
}

func createDeployment(name string, targetLXC string, composeData []byte) error {
	c := client.LyxiClient("/tmp/lixy.sock")

	resp, err := c.SendCommand("create-deployment", map[string]string{
		"name":         name,
		"target_lxc":   targetLXC,
		"compose_yaml": string(composeData),
	})

	if resp.Success != true {
		return errors.New("failed to create deployment: " + resp.Message)
	}
	if err != nil {
		return err
	}
	return nil
}

func init() {
	createCmd.AddCommand(createDeploymentCmd)
	// Configure flags for the create command
	createDeploymentCmd.Flags().String("target-lxc", "", "Target LXC container ID")
	createDeploymentCmd.Flags().String("compose-file", "", "Path to Docker Compose file")
}
