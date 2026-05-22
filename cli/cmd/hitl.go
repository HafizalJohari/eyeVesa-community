package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	hitlApprover   string
	hitlAgentID    string
	hitlAction     string
	hitlResourceID string
	hitlRiskLevel  string
	hitlReason     string
)

var hitlCmd = &cobra.Command{
	Use:   "hitl",
	Short: "Manage human-in-the-loop approvals",
	Long:  "List, approve, and deny HITL approval requests.",
}

var hitlListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List pending HITL approvals",
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

var hitlStatusCmd = &cobra.Command{
	Use:   "status [approval-id]",
	Short: "Get HITL approval status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetHITLStatus(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var hitlEscalateCmd = &cobra.Command{
	Use:   "escalate",
	Short: "Escalate a HITL request to higher approvers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.EscalateHITL(hitlAgentID, hitlAction, hitlResourceID, hitlRiskLevel, hitlReason)
		if err != nil {
			return err
		}
		printSuccess("Escalation submitted")
		printResult(result)
		return nil
	},
}

var hitlRequestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request a HITL approval",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RequestHITL(hitlAgentID, hitlAction, hitlResourceID, nil, hitlRiskLevel)
		if err != nil {
			return err
		}
		printSuccess("HITL request submitted")
		printResult(result)
		return nil
	},
}

func init() {
	hitlApproveCmd.Flags().StringVar(&hitlApprover, "approver", "cli", "Approver method (cli, email, slack)")
	hitlDenyCmd.Flags().StringVar(&hitlApprover, "approver", "cli", "Approver method")

	hitlEscalateCmd.Flags().StringVar(&hitlAgentID, "agent-id", "", "Agent ID (required)")
	hitlEscalateCmd.Flags().StringVar(&hitlAction, "action", "", "Action (required)")
	hitlEscalateCmd.Flags().StringVar(&hitlResourceID, "resource-id", "", "Resource ID")
	hitlEscalateCmd.Flags().StringVar(&hitlRiskLevel, "risk-level", "medium", "Risk level")
	hitlEscalateCmd.Flags().StringVar(&hitlReason, "reason", "", "Escalation reason")
	_ = hitlEscalateCmd.MarkFlagRequired("agent-id")
	_ = hitlEscalateCmd.MarkFlagRequired("action")

	hitlRequestCmd.Flags().StringVar(&hitlAgentID, "agent-id", "", "Agent ID (required)")
	hitlRequestCmd.Flags().StringVar(&hitlAction, "action", "", "Action (required)")
	hitlRequestCmd.Flags().StringVar(&hitlResourceID, "resource-id", "", "Resource ID")
	hitlRequestCmd.Flags().StringVar(&hitlRiskLevel, "risk-level", "medium", "Risk level")
	_ = hitlRequestCmd.MarkFlagRequired("agent-id")
	_ = hitlRequestCmd.MarkFlagRequired("action")

	hitlCmd.AddCommand(hitlListCmd)
	hitlCmd.AddCommand(hitlApproveCmd)
	hitlCmd.AddCommand(hitlDenyCmd)
	hitlCmd.AddCommand(hitlStatusCmd)
	hitlCmd.AddCommand(hitlEscalateCmd)
	hitlCmd.AddCommand(hitlRequestCmd)
	rootCmd.AddCommand(hitlCmd)
}
