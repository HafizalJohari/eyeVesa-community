package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage gateway key rotation",
	Long: `Manage Ed25519 gateway key rotation, inspect rotation status,
and clear previous keys after grace period expiry.`,
}

var keysRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate the gateway signing key",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Post("/v1/keys/rotate", nil)
		if err != nil {
			return fmt.Errorf("rotate key: %w", err)
		}
		printResult(result)
		return nil
	},
}

var keysStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get key rotation status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Get("/v1/keys/status")
		if err != nil {
			return fmt.Errorf("key status: %w", err)
		}
		printResult(result)
		return nil
	},
}

var keysClearPreviousCmd = &cobra.Command{
	Use:   "clear-previous",
	Short: "Clear the previous key (ends grace period)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Post("/v1/keys/clear-previous", nil)
		if err != nil {
			return fmt.Errorf("clear previous key: %w", err)
		}
		printResult(result)
		return nil
	},
}

func init() {
	keysCmd.AddCommand(keysRotateCmd)
	keysCmd.AddCommand(keysStatusCmd)
	keysCmd.AddCommand(keysClearPreviousCmd)
	addAdvancedCommand(keysCmd)
}
