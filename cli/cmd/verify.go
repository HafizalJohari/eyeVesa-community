package cmd

import (
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	verifyAgentID string
	verifyMessage string
	verifySig     string
)

var verifyCmd = &cobra.Command{
	Use:   "verify-signature",
	Short: "Verify an agent's Ed25519 signature",
	Long: `Verify that a message was signed by an agent's Ed25519 private key.

Examples:
  eyevesa verify-signature --agent-id <id> --message "hello" --signature <base64-sig>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		msgBytes := []byte(verifyMessage)
		sigBytes, err := base64.StdEncoding.DecodeString(verifySig)
		if err != nil {
			return fmt.Errorf("invalid base64 signature: %w", err)
		}

		result, err := client.VerifySignature(verifyAgentID, msgBytes, sigBytes)
		if err != nil {
			return err
		}

		valid, _ := result["valid"].(bool)
		if valid {
			printSuccess("Signature is valid")
		} else {
			printError("Signature is INVALID")
		}
		printResult(result)
		return nil
	},
}

func init() {
	verifyCmd.Flags().StringVar(&verifyAgentID, "agent-id", "", "Agent ID (required)")
	verifyCmd.Flags().StringVar(&verifyMessage, "message", "", "Message that was signed (required)")
	verifyCmd.Flags().StringVar(&verifySig, "signature", "", "Base64-encoded signature (required)")

	_ = verifyCmd.MarkFlagRequired("agent-id")
	_ = verifyCmd.MarkFlagRequired("message")
	_ = verifyCmd.MarkFlagRequired("signature")

	rootCmd.AddCommand(verifyCmd)
}