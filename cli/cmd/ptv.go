package cmd

import (
	"github.com/spf13/cobra"
)

var (
	ptvAgentID        string
	ptvPlatform       string
	ptvFirmware       string
	ptvRuntimeHash    string
	ptvBindingID      string
)

var ptvCmd = &cobra.Command{
	Use:   "ptv",
	Short: "Prove-Transform-Verify identity operations",
	Long:  "Attest, bind, and verify hardware-rooted agent identities via the PTV protocol.",
}

var ptvAttestCmd = &cobra.Command{
	Use:   "attest",
	Short: "Attest agent identity (PROVE phase)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.AttestPTV(ptvAgentID, ptvPlatform, ptvFirmware, nil, nil)
		if err != nil {
			return err
		}
		printSuccess("Attestation submitted")
		printResult(result)
		return nil
	},
}

var ptvVerifyCmd = &cobra.Command{
	Use:   "verify [binding-id]",
	Short: "Verify an identity binding (VERIFY phase)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.VerifyPTV(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	ptvAttestCmd.Flags().StringVar(&ptvAgentID, "agent-id", "", "Agent ID (required)")
	ptvAttestCmd.Flags().StringVar(&ptvPlatform, "platform", "macos-arm64", "Platform identifier")
	ptvAttestCmd.Flags().StringVar(&ptvFirmware, "firmware", "1.0.0", "Firmware version")
	ptvAttestCmd.Flags().StringVar(&ptvRuntimeHash, "runtime-hash", "", "Runtime hash")

	_ = ptvAttestCmd.MarkFlagRequired("agent-id")

	ptvCmd.AddCommand(ptvAttestCmd)
	ptvCmd.AddCommand(ptvVerifyCmd)
	rootCmd.AddCommand(ptvCmd)
}