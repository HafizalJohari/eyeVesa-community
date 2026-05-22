package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover [capability]",
	Short: "Discover available tools and resources",
	Args:  cobra.MaximumNArgs(1),
	Long: `Discover available tools and resources registered with the gateway.

Examples:
  eyevesa discover
  eyevesa discover deployment
  eyevesa discover database`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		fmt.Println("Resources:")
		result, err := client.ListResources()
		if err != nil {
			return fmt.Errorf("list resources: %w", err)
		}

		if resources, ok := result["resources"].([]interface{}); ok {
			if len(resources) == 0 {
				fmt.Println("  (none)")
			}
			for _, r := range resources {
				if m, ok := r.(map[string]interface{}); ok {
					name, _ := m["name"].(string)
					rtype, _ := m["resource_type"].(string)
					endpoint, _ := m["endpoint"].(string)
					risk, _ := m["risk_level"].(string)
					fmt.Printf("  %-25s %-12s %-8s %s\n", name, rtype, risk, endpoint)
				}
			}
		}

		fmt.Println("\nMCP Tools:")
		mcpResult, err := client.MCPToolsList()
		if err != nil {
			fmt.Println("  (MCP connection failed)")
			return nil
		}
		if tools, ok := mcpResult["tools"].([]interface{}); ok {
			if len(tools) == 0 {
				fmt.Println("  (none)")
			}
			for _, t := range tools {
				if m, ok := t.(map[string]interface{}); ok {
					name, _ := m["name"].(string)
					fmt.Printf("  %s\n", name)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}
