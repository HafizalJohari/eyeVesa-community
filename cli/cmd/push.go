package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	pushAgentID  string
	pushToken    string
	pushPlatform string
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Manage push notification tokens",
	Long:  "Register, list, and deactivate push notification device tokens.",
}

var pushRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a push notification token",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RegisterPushToken(pushAgentID, pushToken, pushPlatform)
		if err != nil {
			return err
		}
		printSuccess("Push token registered")
		printResult(result)
		return nil
	},
}

var pushListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List registered push tokens",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetPushTokens()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var pushDeactivateCmd = &cobra.Command{
	Use:   "deactivate [token-id]",
	Short: "Deactivate a push token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		_, err := client.DeactivatePushToken(args[0])
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Push token deactivated: %s", args[0]))
		return nil
	},
}

func init() {
	pushRegisterCmd.Flags().StringVar(&pushAgentID, "agent-id", "", "Agent ID (required)")
	pushRegisterCmd.Flags().StringVar(&pushToken, "token", "", "Device token (required)")
	pushRegisterCmd.Flags().StringVar(&pushPlatform, "platform", "apns", "Platform: apns, fcm")
	_ = pushRegisterCmd.MarkFlagRequired("agent-id")
	_ = pushRegisterCmd.MarkFlagRequired("token")

	pushCmd.AddCommand(pushRegisterCmd)
	pushCmd.AddCommand(pushListCmd)
	pushCmd.AddCommand(pushDeactivateCmd)
	rootCmd.AddCommand(pushCmd)
}
