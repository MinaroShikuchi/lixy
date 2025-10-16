package main

import (
	"fmt"
	"log"
	"os"

	"github.com/MinaroShikuchi/lixy/internal/client"
	"github.com/MinaroShikuchi/lixy/internal/store"

	"github.com/spf13/cobra"
)

var tokenStore *store.TokenStore

func main() {
	var err error
	tokenStore, err = store.NewTokenStore()
	if err != nil {
		log.Fatalf("Failed to initialize token store: %v", err)
	}
	defer tokenStore.Close()

	if err := Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

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

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a lixy controller as a worker agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		controller, _ := cmd.Flags().GetString("controller")
		name, _ := cmd.Flags().GetString("name")

		if token == "" {
			return fmt.Errorf("registration token is required (--token)")
		}

		if controller == "" {
			return fmt.Errorf("controller URL is required (--controller)")
		}

		if name == "" {
			// Use hostname as default
			hostname, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("failed to get hostname: %w", err)
			}
			name = hostname
		}

		fmt.Printf("Registering with controller at %s...\n", controller)
		if err := client.RegisterWithController(*tokenStore, controller, token, name); err != nil {
			return err
		}

		fmt.Println("Successfully registered! This agent will now accept commands from the controller.")
		return nil
	},
}
