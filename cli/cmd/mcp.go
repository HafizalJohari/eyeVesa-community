package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP (Model Context Protocol) operations",
	Long: `MCP allows AI agents to discover and call tools through registered resource adapters.

This is the execution layer that sits on top of KYA:
- KYA = Identity & Trust
- Airport = Discovery
- MCP = Tool Execution

Use with registered MCP servers (see resource-adapter-go).`,
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
	mcpCallArgs    string
)

var mcpCallCmd = &cobra.Command{
	Use:   "call",
	Short: "Call an MCP tool",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		var arguments map[string]interface{}
		if mcpCallArgs != "" {
			if err := json.Unmarshal([]byte(mcpCallArgs), &arguments); err != nil {
				return fmt.Errorf("invalid arguments JSON: %w", err)
			}
		}
		result, err := client.MCPCallTool(mcpCallAgentID, mcpCallTool, arguments)
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
	mcpCallCmd.Flags().StringVar(&mcpCallArgs, "args", "", "Tool arguments as JSON (e.g. '{\"agent_id\":\"...\"}')")
	_ = mcpCallCmd.MarkFlagRequired("agent-id")
	_ = mcpCallCmd.MarkFlagRequired("tool")

	mcpCmd.AddCommand(mcpInitCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpCallCmd)
	rootCmd.AddCommand(mcpCmd)
}