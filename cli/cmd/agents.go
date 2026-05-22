package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agents",
	Long:  "List, get, and inspect registered agents.",
}

var agentsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all agents",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListAgents()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var agentsGetCmd = &cobra.Command{
	Use:   "get [agent-id]",
	Short: "Get agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetAgent(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var agentsTrustCmd = &cobra.Command{
	Use:   "trust [agent-id]",
	Short: "View agent trust score",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetAgent(args[0])
		if err != nil {
			return err
		}
		name, _ := result["name"].(string)
		trustScore := result["trust_score"]
		status, _ := result["status"].(string)
		fmt.Printf("Agent: %s (%s)\n", name, args[0])
		fmt.Printf("  Trust score: %.4f\n", trustScore)
		fmt.Printf("  Status:      %s\n", status)
		return nil
	},
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsGetCmd)
	agentsCmd.AddCommand(agentsTrustCmd)
	rootCmd.AddCommand(agentsCmd)
}
