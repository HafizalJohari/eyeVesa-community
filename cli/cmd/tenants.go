package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	tenantName string
	tenantDesc string
)

var tenantsCmd = &cobra.Command{
	Use:   "tenants",
	Short: "Manage multi-tenant configuration",
	Long:  "Create, list, and inspect tenants in the gateway.",
}

var tenantsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.CreateTenant(tenantName, tenantDesc)
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Tenant created: %s", tenantName))
		printResult(result)
		return nil
	},
}

var tenantsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all tenants",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListTenants()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var tenantsGetCmd = &cobra.Command{
	Use:   "get [tenant-id]",
	Short: "Get tenant details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetTenant(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	tenantsCreateCmd.Flags().StringVarP(&tenantName, "name", "n", "", "Tenant name (required)")
	tenantsCreateCmd.Flags().StringVarP(&tenantDesc, "description", "d", "", "Tenant description")
	_ = tenantsCreateCmd.MarkFlagRequired("name")

	tenantsCmd.AddCommand(tenantsCreateCmd)
	tenantsCmd.AddCommand(tenantsListCmd)
	tenantsCmd.AddCommand(tenantsGetCmd)
	rootCmd.AddCommand(tenantsCmd)
}
