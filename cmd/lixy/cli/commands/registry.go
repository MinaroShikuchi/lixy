package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/client/socket"
	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage container registry credentials",
	Long:  `Store, list, and manage container registry credentials for secure image pulling`,
}

var registryAddCmd = &cobra.Command{
	Use:   "add [registry]",
	Short: "Add registry credentials",
	Long: `Add credentials for a container registry.

Currently supported registries:
  - ghcr (GitHub Container Registry)

Examples:
  # Interactive (prompts for token)
  lixy registry add ghcr --username myorg

  # From environment variable
  lixy registry add ghcr --username myorg --token-env GITHUB_TOKEN

  # From file
  lixy registry add ghcr --username myorg --token-file ~/.github-token`,
	Args: cobra.ExactArgs(1),
	RunE: runRegistryAdd,
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored registry credentials",
	Long:  `List all stored registry credentials (tokens are not displayed for security)`,
	RunE:  runRegistryList,
}

var registryDeleteCmd = &cobra.Command{
	Use:   "delete [registry]",
	Short: "Delete registry credentials",
	Long:  `Delete stored credentials for a registry`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRegistryDelete,
}

var (
	registryUsername  string
	registryTokenEnv  string
	registryTokenFile string
)

func init() {
	rootCmd.AddCommand(registryCmd)
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryListCmd)
	registryCmd.AddCommand(registryDeleteCmd)

	// Flags for add command
	registryAddCmd.Flags().StringVar(&registryUsername, "username", "", "Registry username (required)")
	registryAddCmd.Flags().StringVar(&registryTokenEnv, "token-env", "", "Environment variable containing token")
	registryAddCmd.Flags().StringVar(&registryTokenFile, "token-file", "", "File containing token")
	registryAddCmd.MarkFlagRequired("username")
}

func runRegistryAdd(cmd *cobra.Command, args []string) error {
	registry := args[0]

	// Validate registry type
	if registry != "ghcr" {
		return fmt.Errorf("unsupported registry type: %s (currently only 'ghcr' is supported)", registry)
	}

	// Get token from various sources
	var token string

	if registryTokenEnv != "" {
		// Get token from environment variable
		token = os.Getenv(registryTokenEnv)
		if token == "" {
			return fmt.Errorf("environment variable %s is not set or empty", registryTokenEnv)
		}
	} else if registryTokenFile != "" {
		// Get token from file
		tokenBytes, err := os.ReadFile(registryTokenFile)
		if err != nil {
			return fmt.Errorf("failed to read token file: %w", err)
		}
		token = strings.TrimSpace(string(tokenBytes))
		if token == "" {
			return fmt.Errorf("token file is empty")
		}
	} else {
		// Interactive prompt for token
		fmt.Print("Enter registry token: ")
		reader := bufio.NewReader(os.Stdin)
		tokenInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(tokenInput)
		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}
	}

	// Send command via socket
	client := socket.NewControllerClient()
	params := domain.AddRegistryCredentialOptions{
		RegistryType: registry,
		Username:     registryUsername,
		Token:        token,
	}

	resp, err := client.SendCommand("add-registry-credential", params)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to store credentials: %s", resp.Message)
	}

	fmt.Printf("✓ Successfully stored credentials for registry: %s\n", registry)
	fmt.Printf("  Username: %s\n", registryUsername)
	return nil
}

func runRegistryList(cmd *cobra.Command, args []string) error {
	// Send command via socket
	client := socket.NewControllerClient()
	resp, err := client.SendCommand("list-registry-credentials", nil)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to list credentials: %s", resp.Message)
	}

	// Parse credentials from response
	credentials, ok := resp.Data.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	if len(credentials) == 0 {
		fmt.Println("No registry credentials stored")
		return nil
	}

	// Display credentials in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "REGISTRY\tUSERNAME\tCREATED\tUPDATED")
	fmt.Fprintln(w, "--------\t--------\t-------\t-------")

	for _, item := range credentials {
		cred, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		registryType := cred["registry_type"].(string)
		username := cred["username"].(string)

		// Parse timestamps
		createdAt := "N/A"
		if createdStr, ok := cred["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				createdAt = t.Format("2006-01-02 15:04")
			}
		}

		updatedAt := "N/A"
		if updatedStr, ok := cred["updated_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
				updatedAt = t.Format("2006-01-02 15:04")
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			registryType,
			username,
			createdAt,
			updatedAt,
		)
	}

	w.Flush()
	return nil
}

func runRegistryDelete(cmd *cobra.Command, args []string) error {
	registry := args[0]

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete credentials for registry '%s'? (y/N): ", registry)
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("Deletion cancelled")
		return nil
	}

	// Send command via socket
	client := socket.NewControllerClient()
	params := domain.DeleteRegistryCredentialOptions{
		RegistryType: registry,
	}

	resp, err := client.SendCommand("delete-registry-credential", params)
	if err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("failed to delete credentials: %s", resp.Message)
	}

	fmt.Printf("✓ Successfully deleted credentials for registry: %s\n", registry)
	return nil
}
