package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	authzAgentID    string
	authzAction     string
	authzResourceID string
	authzParams     string
)

var authorizeCmd = &cobra.Command{
	Use:     "authorize",
	Short:   "Authorize an agent action",
	Aliases: []string{"auth"},
	Long: `Check whether an agent is authorized to perform an action.

Examples:
  eyevesa authorize --agent-id <id> --action database_query
  eyevesa authorize --agent-id <id> --action bank_transfer --resource-id <id> --params '{"amount": 250}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		var params map[string]interface{}
		if authzParams != "" {
			// simple key=val parsing
			params = map[string]interface{}{
				"raw": authzParams,
			}
		}

		result, err := client.Authorize(authzAgentID, authzAction, authzResourceID, params)
		if err != nil {
			return err
		}

		allowed, _ := result["allowed"].(bool)
		requiresHITL, _ := result["requires_hitl"].(bool)
		reason, _ := result["reason"].(string)
		trustDelta := result["trust_delta"]

		if allowed {
			printSuccess(fmt.Sprintf("ALLOWED: %s", reason))
		} else if requiresHITL {
			fmt.Printf("⚠ HITL REQUIRED: %s\n", reason)
		} else {
			printError(fmt.Sprintf("DENIED: %s", reason))
		}

		fmt.Printf("  Trust delta: %.2f\n", trustDelta)
		return nil
	},
}

func init() {
	authorizeCmd.Flags().StringVar(&authzAgentID, "agent-id", "", "Agent ID (required)")
	authorizeCmd.Flags().StringVar(&authzAction, "action", "", "Action/tool name (required)")
	authorizeCmd.Flags().StringVar(&authzResourceID, "resource-id", "", "Resource ID")
	authorizeCmd.Flags().StringVarP(&authzParams, "params", "p", "", "Action parameters (JSON or key=val)")

	_ = authorizeCmd.MarkFlagRequired("agent-id")
	_ = authorizeCmd.MarkFlagRequired("action")

	rootCmd.AddCommand(authorizeCmd)
}
