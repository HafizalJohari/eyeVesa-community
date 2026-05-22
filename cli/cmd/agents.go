package cmd

import (
	"fmt"
	"strings"

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

var skipDeleteConfirmation bool

var agentsDeleteCmd = &cobra.Command{
	Use:   "delete [agent-id]",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]
		if !skipDeleteConfirmation {
			fmt.Printf("Delete agent %s? Type 'yes' to confirm: ", agentID)
			var confirm string
			if _, err := fmt.Scanln(&confirm); err != nil {
				return fmt.Errorf("confirmation failed: %w", err)
			}
			if strings.ToLower(strings.TrimSpace(confirm)) != "yes" {
				return fmt.Errorf("delete cancelled")
			}
		}

		client := getClient()
		result, err := client.DeleteAgent(agentID)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsGetCmd)
	agentsCmd.AddCommand(agentsTrustCmd)
	agentsDeleteCmd.Flags().BoolVarP(&skipDeleteConfirmation, "yes", "y", false, "skip confirmation prompt")
	agentsCmd.AddCommand(agentsDeleteCmd)
	addCoreCommand(agentsCmd)
}
