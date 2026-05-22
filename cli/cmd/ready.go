package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	readinessEndpoint string
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Check gateway readiness",
	Long:  "Check if the gateway is ready to accept requests (not draining).",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Ready()
		if err != nil {
			return err
		}
		status, _ := result["status"].(float64)
		if int(status) == 200 {
			printSuccess("Gateway is ready")
		} else {
			printError(fmt.Sprintf("Gateway not ready (HTTP %d)", int(status)))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(readyCmd)
}
