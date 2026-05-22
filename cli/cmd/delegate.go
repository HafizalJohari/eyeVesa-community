package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	delegateParentID string
	delegateChildID  string
	delegateScope    []string
	delegateDepth    int
	delegateDuration string
)

var delegateCmd = &cobra.Command{
	Use:   "delegate",
	Short: "Manage agent-to-agent delegation",
	Long: `Create, validate, and revoke agent delegations.

Examples:
  eyevesa delegate create --parent <id> --child <id> --scope "database_query,log_search" --depth 1 --duration 2h
  eyevesa delegate validate --parent <id> --child <id>
  eyevesa delegate list <agent-id>
  eyevesa delegate revoke <delegation-id>`,
}

var delegateCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a delegation",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Delegate(delegateParentID, delegateChildID, delegateScope, delegateDepth, delegateDuration)
		if err != nil {
			return err
		}
		printSuccess("Delegation created")
		printResult(result)
		return nil
	},
}

var delegateValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a delegation",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ValidateDelegation(delegateParentID, delegateChildID)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var delegateListCmd = &cobra.Command{
	Use:     "list [agent-id]",
	Short:   "List delegations for an agent",
	Aliases: []string{"ls"},
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListDelegations(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var delegateRevokeCmd = &cobra.Command{
	Use:   "revoke [delegation-id]",
	Short: "Revoke a delegation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RevokeDelegation(args[0])
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Delegation revoked: %s", args[0]))
		printResult(result)
		return nil
	},
}

func init() {
	delegateCreateCmd.Flags().StringVar(&delegateParentID, "parent", "", "Parent agent ID (required)")
	delegateCreateCmd.Flags().StringVar(&delegateChildID, "child", "", "Child agent ID (required)")
	delegateCreateCmd.Flags().StringSliceVar(&delegateScope, "scope", []string{}, "Delegation scope (comma-separated)")
	delegateCreateCmd.Flags().IntVar(&delegateDepth, "depth", 1, "Max delegation depth")
	delegateCreateCmd.Flags().StringVar(&delegateDuration, "duration", "1h", "Delegation duration (e.g. 2h30m, 24h)")

	_ = delegateCreateCmd.MarkFlagRequired("parent")
	_ = delegateCreateCmd.MarkFlagRequired("child")

	delegateValidateCmd.Flags().StringVar(&delegateParentID, "parent", "", "Parent agent ID (required)")
	delegateValidateCmd.Flags().StringVar(&delegateChildID, "child", "", "Child agent ID (required)")

	_ = delegateValidateCmd.MarkFlagRequired("parent")
	_ = delegateValidateCmd.MarkFlagRequired("child")

	delegateCmd.AddCommand(delegateCreateCmd)
	delegateCmd.AddCommand(delegateValidateCmd)
	delegateCmd.AddCommand(delegateListCmd)
	delegateCmd.AddCommand(delegateRevokeCmd)
	rootCmd.AddCommand(delegateCmd)
}
