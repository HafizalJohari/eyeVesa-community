package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Transaction protocol operations",
	Long: `Manage capability tokens, verify transactions, and issue receipts.

The transaction protocol enables authenticated, non-repudiable interactions
between AI agents and enterprise resources via gateway-signed capability tokens.`,
}

var (
	txAgentID    string
	txResourceID string
	txAction     string
	txScopes     string
	txToken      string
	txTokenID    string
	txReason     string
	txReceipt    string
)

var txIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Issue a capability token for an agent",
	Long: `Request a capability token from the gateway. The gateway evaluates
policy, updates trust, and returns a signed token if authorized.

Examples:
  eyevesa tx issue --agent-id <uuid> --resource-id <uuid> --action deploy
  eyevesa tx issue --agent-id <uuid> --action read --scopes "read,write"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var scopes []string
		if txScopes != "" {
			scopes = parseList(txScopes)
		}
		result, err := client.IssueCapabilityToken(txAgentID, txResourceID, txAction, scopes, nil)
		if err != nil {
			return err
		}
		if allowed, ok := result["allowed"].(bool); ok && !allowed {
			printError("Authorization denied")
			if reason, ok := result["reason"].(string); ok {
				printKeyValue("Reason", reason)
			}
			return nil
		}
		printSuccess("Capability token issued")
		if ct, ok := result["capability_token"].(map[string]interface{}); ok {
			printKeyValue("Token ID", fmt.Sprintf("%v", ct["jti"]))
			printKeyValue("Action", fmt.Sprintf("%v", ct["action"]))
			printKeyValue("Expires", fmt.Sprintf("%v", ct["exp"]))
		}
		printResult(result)
		return nil
	},
}

var txVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify a capability token",
	Long: `Verify that a capability token is valid, not expired, and not revoked.

Examples:
  eyevesa tx verify --token <token-json>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.VerifyCapabilityToken(txToken)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var txRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke a capability token",
	Long: `Revoke a capability token by its ID. Revoked tokens cannot be used
even if they have not expired.

Examples:
  eyevesa tx revoke --token-id <jti> --reason "security incident"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RevokeCapabilityToken(txTokenID, txReason)
		if err != nil {
			return err
		}
		printSuccess("Token revoked")
		printResult(result)
		return nil
	},
}

var txRevokedCmd = &cobra.Command{
	Use:     "revoked",
	Short:   "List revoked tokens",
	Aliases: []string{"ls-revoked"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListRevokedTokens()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var txReceiptCmd = &cobra.Command{
	Use:   "receipt",
	Short: "Issue a transaction receipt",
	Long: `Present a capability token to the gateway and receive a signed
transaction receipt as proof that the transaction occurred.

Examples:
  eyevesa tx receipt --token <token-json>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.IssueTransactionReceipt(txToken)
		if err != nil {
			return err
		}
		printSuccess("Transaction receipt issued")
		printResult(result)
		return nil
	},
}

var txVerifyReceiptCmd = &cobra.Command{
	Use:   "verify-receipt",
	Short: "Verify a transaction receipt",
	Long: `Verify the signature and validity of a transaction receipt.

Examples:
  eyevesa tx verify-receipt --receipt <receipt-json>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.VerifyTransactionReceipt(txReceipt)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	txIssueCmd.Flags().StringVar(&txAgentID, "agent-id", "", "Agent ID (required)")
	txIssueCmd.Flags().StringVar(&txResourceID, "resource-id", "", "Resource ID")
	txIssueCmd.Flags().StringVar(&txAction, "action", "", "Action to authorize (required)")
	txIssueCmd.Flags().StringVar(&txScopes, "scopes", "", "Comma-separated scopes")
	_ = txIssueCmd.MarkFlagRequired("agent-id")
	_ = txIssueCmd.MarkFlagRequired("action")

	txVerifyCmd.Flags().StringVar(&txToken, "token", "", "Capability token JSON (required)")
	_ = txVerifyCmd.MarkFlagRequired("token")

	txRevokeCmd.Flags().StringVar(&txTokenID, "token-id", "", "Token ID (jti) to revoke (required)")
	txRevokeCmd.Flags().StringVar(&txReason, "reason", "revoked by administrator", "Revocation reason")
	_ = txRevokeCmd.MarkFlagRequired("token-id")

	txReceiptCmd.Flags().StringVar(&txToken, "token", "", "Capability token JSON (required)")
	_ = txReceiptCmd.MarkFlagRequired("token")

	txVerifyReceiptCmd.Flags().StringVar(&txReceipt, "receipt", "", "Receipt JSON (required)")
	_ = txVerifyReceiptCmd.MarkFlagRequired("receipt")

	txCmd.AddCommand(txIssueCmd)
	txCmd.AddCommand(txVerifyCmd)
	txCmd.AddCommand(txRevokeCmd)
	txCmd.AddCommand(txRevokedCmd)
	txCmd.AddCommand(txReceiptCmd)
	txCmd.AddCommand(txVerifyReceiptCmd)
	addAdvancedCommand(txCmd)
}
