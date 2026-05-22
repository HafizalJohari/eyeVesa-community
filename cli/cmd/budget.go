package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	budgetAgentID  string
	budgetAmount   float64
	budgetCurrency string
	budgetCategory string
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Budget metering operations",
	Long:  "Check and record agent budget spending.",
}

var budgetCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check agent budget status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.CheckBudget(budgetAgentID)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var budgetSpendCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a budget spend",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RecordSpend(budgetAgentID, budgetAmount, budgetCurrency, budgetCategory)
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Spend recorded: %.2f %s", budgetAmount, budgetCurrency))
		printResult(result)
		return nil
	},
}

func init() {
	budgetCheckCmd.Flags().StringVar(&budgetAgentID, "agent-id", "", "Agent ID (required)")
	_ = budgetCheckCmd.MarkFlagRequired("agent-id")

	budgetSpendCmd.Flags().StringVar(&budgetAgentID, "agent-id", "", "Agent ID (required)")
	budgetSpendCmd.Flags().Float64Var(&budgetAmount, "amount", 0, "Spend amount (required)")
	budgetSpendCmd.Flags().StringVar(&budgetCurrency, "currency", "USD", "Currency code")
	budgetSpendCmd.Flags().StringVar(&budgetCategory, "category", "general", "Spend category")
	_ = budgetSpendCmd.MarkFlagRequired("agent-id")
	_ = budgetSpendCmd.MarkFlagRequired("amount")

	budgetCmd.AddCommand(budgetCheckCmd)
	budgetCmd.AddCommand(budgetSpendCmd)
	rootCmd.AddCommand(budgetCmd)
}
