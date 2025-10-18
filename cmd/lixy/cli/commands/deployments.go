package commands

import (
	"fmt"

	"github.com/MinaroShikuchi/lixy/pkg/client"
	"github.com/spf13/cobra"
)

// deploymentsCmd represents the deployments subcommand of get
var deploymentsCmd = &cobra.Command{
	Use:   "deployments [NAME]",
	Short: "List all deployments or get details of a specific deployment",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.LyxiClient("/tmp/lixy.sock")

		// If a name is provided, get details for that specific deployment
		if len(args) > 0 {
			deploymentName := args[0]
			params := map[string]string{"name": deploymentName}

			resp, err := c.SendCommand("get-deployment", params)
			if err != nil {
				return err
			}

			if !resp.Success {
				return fmt.Errorf("error: %s", resp.Message)
			}

			// Display detailed information for the specified deployment
			deployment, ok := resp.Data.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid response format")
			}

			fmt.Printf("Name: %s\n", deployment["name"])
			fmt.Printf("Target LXC: %s\n", deployment["targetLXC"])
			fmt.Printf("Status: %s\n", deployment["status"])
			// Display other deployment details

			return nil
		}

		// Otherwise list all deployments
		deployments, err := c.SendCommand("get-deployments", nil)
		if err != nil {
			return err
		}

		if !deployments.Success {
			return fmt.Errorf("error: %s", deployments.Message)
		}

		// Parse and display deployments
		items, ok := deployments.Data.([]interface{})
		if !ok {
			return fmt.Errorf("invalid response format")
		}

		fmt.Println("DEPLOYMENTS:")
		fmt.Println("NAME\t\tTARGET LXC\tSTATUS")
		for _, item := range items {
			deployment, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			fmt.Printf("%s\t\t%s\t\t%s\n",
				deployment["name"],
				deployment["targetLXC"],
				deployment["status"])
		}

		return nil
	},
}
