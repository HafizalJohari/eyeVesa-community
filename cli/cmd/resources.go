package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var resourcesCmd = &cobra.Command{
	Use:   "resources",
	Short: "Manage resources",
	Long:  "List, get, and register enterprise resources.",
}

var resourcesListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all resources",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListResources()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var resourcesGetCmd = &cobra.Command{
	Use:   "get [resource-id]",
	Short: "Get resource details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetResource(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var (
	resourceName         string
	resourceType         string
	resourceEndpoint     string
	resourceAuthMethod   string
	resourceRiskLevel    string
	resourceDataSens     string
	resourceRateLimit    int
	resourceCapabilities string
)

var resourcesRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new resource",
	Long: `Register a new enterprise resource with the eyeVesa Gateway.

Examples:
  eyevesa resources register --name k8s-api --type mcp_server --endpoint https://k8s-adapter:8443
  eyevesa resources register --name analytics-db --type mcp_server --endpoint https://db:8443 --risk-level high`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()

		var caps interface{}
		if resourceCapabilities != "" {
			caps = fmt.Sprintf(`{"desc": "%s"}`, resourceCapabilities)
		}

		result, err := client.RegisterResource(
			resourceName,
			resourceType,
			resourceEndpoint,
			resourceAuthMethod,
			resourceRiskLevel,
			resourceDataSens,
			resourceRateLimit,
			caps,
		)
		if err != nil {
			return err
		}

		printSuccess(fmt.Sprintf("Resource registered: %s", resourceName))
		printKeyValue("Resource ID", fmt.Sprintf("%v", result["resource_id"]))
		printKeyValue("Type", resourceType)
		printKeyValue("Endpoint", resourceEndpoint)
		printKeyValue("Risk level", resourceRiskLevel)
		return nil
	},
}

func init() {
	resourcesRegisterCmd.Flags().StringVarP(&resourceName, "name", "n", "", "Resource name (required)")
	resourcesRegisterCmd.Flags().StringVarP(&resourceType, "type", "t", "mcp_server", "Resource type")
	resourcesRegisterCmd.Flags().StringVarP(&resourceEndpoint, "endpoint", "e", "", "Resource endpoint URL (required)")
	resourcesRegisterCmd.Flags().StringVar(&resourceAuthMethod, "auth-method", "mTLS+SVID", "Authentication method")
	resourcesRegisterCmd.Flags().StringVar(&resourceRiskLevel, "risk-level", "medium", "Risk level: low, medium, high, critical")
	resourcesRegisterCmd.Flags().StringVar(&resourceDataSens, "data-sensitivity", "internal", "Data sensitivity: public, internal, confidential, restricted")
	resourcesRegisterCmd.Flags().IntVar(&resourceRateLimit, "rate-limit", 100, "Rate limit per agent")
	resourcesRegisterCmd.Flags().StringVar(&resourceCapabilities, "capabilities", "", "Capabilities description")

	_ = resourcesRegisterCmd.MarkFlagRequired("name")
	_ = resourcesRegisterCmd.MarkFlagRequired("endpoint")

	resourcesCmd.AddCommand(resourcesListCmd)
	resourcesCmd.AddCommand(resourcesGetCmd)
	resourcesCmd.AddCommand(resourcesRegisterCmd)
	rootCmd.AddCommand(resourcesCmd)
}
