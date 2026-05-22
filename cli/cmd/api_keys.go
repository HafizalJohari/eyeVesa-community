package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	apiKeyName     string
	apiKeyTenantID string
)

var apiKeysCmd = &cobra.Command{
	Use:   "api-keys",
	Short: "Manage API keys",
	Long:  "Create, list, and revoke API keys for agent authentication.",
}

var apiKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.CreateAPIKey(apiKeyName, apiKeyTenantID)
		if err != nil {
			return fmt.Errorf("create api key: %w", err)
		}
		printSuccess("API key created")
		printResult(result)
		return nil
	},
}

var apiKeysListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListAPIKeys()
		if err != nil {
			return fmt.Errorf("list api keys: %w", err)
		}
		printResult(result)
		return nil
	},
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke [key-id]",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RevokeAPIKey(args[0])
		if err != nil {
			return fmt.Errorf("revoke api key: %w", err)
		}
		printSuccess("API key revoked")
		printResult(result)
		return nil
	},
}

func init() {
	apiKeysCreateCmd.Flags().StringVarP(&apiKeyName, "name", "n", "", "Key name (required)")
	apiKeysCreateCmd.Flags().StringVarP(&apiKeyTenantID, "tenant-id", "t", "", "Tenant ID (e.g. org:phos)")
	_ = apiKeysCreateCmd.MarkFlagRequired("name")

	apiKeysCmd.AddCommand(apiKeysCreateCmd)
	apiKeysCmd.AddCommand(apiKeysListCmd)
	apiKeysCmd.AddCommand(apiKeysRevokeCmd)
	addOperateCommand(apiKeysCmd)
}
