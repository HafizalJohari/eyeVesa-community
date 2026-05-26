package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

var federationCmd = &cobra.Command{
	Use:   "federation",
	Short: "Manage trusted community agent nodes",
	Long:  "Invite, register, inspect, and sync trusted eyeVesa community nodes for secure federated agent discovery.",
}

var federationPeersCmd = &cobra.Command{
	Use:     "peers",
	Short:   "List trusted federation peers",
	Aliases: []string{"list"},
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := getClient().FederationListPeers()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var federationInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Create an invite token for a community node",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		endpoint, _ := cmd.Flags().GetString("endpoint")
		trustDomain, _ := cmd.Flags().GetString("trust-domain")
		peerType, _ := cmd.Flags().GetString("peer-type")
		ttlHours, _ := cmd.Flags().GetInt("ttl-hours")
		result, err := getClient().FederationCreateInvite(name, endpoint, trustDomain, peerType, ttlHours)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var federationRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a trusted community node with an invite token",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		publicKey, _ := cmd.Flags().GetString("public-key")
		endpoint, _ := cmd.Flags().GetString("endpoint")
		trustDomain, _ := cmd.Flags().GetString("trust-domain")
		peerType, _ := cmd.Flags().GetString("peer-type")
		inviteToken, _ := cmd.Flags().GetString("invite-token")
		adminApproved, _ := cmd.Flags().GetBool("admin-approved")
		result, err := getClient().FederationRegisterPeer(name, publicKey, endpoint, trustDomain, peerType, inviteToken, adminApproved)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var federationSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync a signed agent passport into this node's federated Airport",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, _ := cmd.Flags().GetString("payload")
		if payload == "" {
			return cmd.Help()
		}
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return err
		}
		result, err := getClient().FederationSyncAgent(body)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	federationInviteCmd.Flags().String("name", "", "Peer node name")
	federationInviteCmd.Flags().String("endpoint", "", "Peer node endpoint, for example http://node-b.local:8080")
	federationInviteCmd.Flags().String("trust-domain", "", "Peer trust domain")
	federationInviteCmd.Flags().String("peer-type", "community", "Peer type")
	federationInviteCmd.Flags().Int("ttl-hours", 24, "Invite lifetime in hours")
	federationInviteCmd.MarkFlagRequired("name")
	federationInviteCmd.MarkFlagRequired("endpoint")

	federationRegisterCmd.Flags().String("name", "", "Peer node name")
	federationRegisterCmd.Flags().String("public-key", "", "Base64 Ed25519 public key")
	federationRegisterCmd.Flags().String("endpoint", "", "Peer node endpoint")
	federationRegisterCmd.Flags().String("trust-domain", "", "Peer trust domain")
	federationRegisterCmd.Flags().String("peer-type", "community", "Peer type")
	federationRegisterCmd.Flags().String("invite-token", "", "Invite token from the receiving node")
	federationRegisterCmd.Flags().Bool("admin-approved", false, "Bypass invite token for admin-controlled local setup")
	federationRegisterCmd.MarkFlagRequired("name")
	federationRegisterCmd.MarkFlagRequired("public-key")
	federationRegisterCmd.MarkFlagRequired("endpoint")

	federationSyncCmd.Flags().String("payload", "", "Raw JSON body for /v1/federation/agents/sync")

	federationCmd.AddCommand(federationPeersCmd)
	federationCmd.AddCommand(federationInviteCmd)
	federationCmd.AddCommand(federationRegisterCmd)
	federationCmd.AddCommand(federationSyncCmd)
	addOperateCommand(federationCmd)
}
