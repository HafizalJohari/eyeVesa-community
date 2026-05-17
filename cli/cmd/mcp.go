package cmd

import (
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP protocol operations",
	Long:  "Interact with the Model Context Protocol endpoint.",
}

var mcpInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize MCP connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.MCPInitialize()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var mcpToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List available MCP tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.MCPToolsList()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var (
	mcpCallAgentID string
	mcpCallTool    string
)

var mcpCallCmd = &cobra.Command{
	Use:   "call",
	Short: "Call an MCP tool",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.MCPCallTool(mcpCallAgentID, mcpCallTool, nil)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	mcpCallCmd.Flags().StringVar(&mcpCallAgentID, "agent-id", "", "Agent ID (required)")
	mcpCallCmd.Flags().StringVar(&mcpCallTool, "tool", "", "Tool name (required)")
	_ = mcpCallCmd.MarkFlagRequired("agent-id")
	_ = mcpCallCmd.MarkFlagRequired("tool")

	mcpCmd.AddCommand(mcpInitCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpCallCmd)
	rootCmd.AddCommand(mcpCmd)
}