package cmd

import (
	"github.com/spf13/cobra"
)

var (
	auditAgentID string
	auditLimit   int
	auditOffset  int
)

var auditCmd = &cobra.Command{
	Use:   "audit [agent-id]",
	Short: "View audit trail for an agent",
	Long: `View the non-repudiable audit trail for an agent.

Examples:
  eyevesa audit hermes-ops
  eyevesa audit hermes-ops --limit 20
  eyevesa audit hermes-ops --limit 50 --offset 100`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Audit(args[0], auditLimit, auditOffset)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	auditCmd.Flags().IntVar(&auditLimit, "limit", 10, "Number of entries to return")
	auditCmd.Flags().IntVar(&auditOffset, "offset", 0, "Offset for pagination")

	rootCmd.AddCommand(auditCmd)
}