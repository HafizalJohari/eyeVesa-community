package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	hitlApprover string
)

var hitlCmd = &cobra.Command{
	Use:   "hitl",
	Short: "Manage human-in-the-loop approvals",
	Long:  "List, approve, and deny HITL approval requests.",
}

var hitlListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending HITL approvals",
	Aliases: []string{"ls", "pending"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListHILTPending()
		if err != nil {
			return fmt.Errorf("list HITL: %w", err)
		}
		printResult(result)
		return nil
	},
}

var hitlApproveCmd = &cobra.Command{
	Use:   "approve [approval-id]",
	Short: "Approve a HITL request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ApproveHILT(args[0], hitlApprover)
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Approved: %s", args[0]))
		printResult(result)
		return nil
	},
}

var hitlDenyCmd = &cobra.Command{
	Use:   "deny [approval-id]",
	Short: "Deny a HITL request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.DenyHILT(args[0], hitlApprover)
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Denied: %s", args[0]))
		printResult(result)
		return nil
	},
}

func init() {
	hitlApproveCmd.Flags().StringVar(&hitlApprover, "approver", "cli", "Approver method (cli, email, slack)")
	hitlDenyCmd.Flags().StringVar(&hitlApprover, "approver", "cli", "Approver method")

	hitlCmd.AddCommand(hitlListCmd)
	hitlCmd.AddCommand(hitlApproveCmd)
	hitlCmd.AddCommand(hitlDenyCmd)
	rootCmd.AddCommand(hitlCmd)
}