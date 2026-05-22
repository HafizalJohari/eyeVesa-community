package cmd

import (
	"fmt"
	"time"

	"github.com/HafizalJohari/eyeVesa-community/cli/internal/config"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Register an agent and keep it online at the Airport",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		owner, _ := cmd.Flags().GetString("owner")
		status, _ := cmd.Flags().GetString("status")
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if owner == "" {
			owner = "community"
		}
		if status == "" {
			status = "online"
		}
		if interval <= 0 {
			interval = 30 * time.Second
		}

		endpoint := gatewayAddr
		if endpoint == "" {
			endpoint = "http://localhost:8080"
		}
		client := getClient()
		client.BaseURL = endpoint
		if client.APIKey == "" && client.JWTToken == "" {
			return fmt.Errorf("connect now requires an existing API key or JWT in config; create or configure one before registering an agent")
		}

		result, err := client.RegisterAgent(name, owner, nil, nil, 0, "", nil)
		if err != nil {
			return err
		}

		agentID, _ := result["agent_id"].(string)
		apiKey, _ := result["api_key"].(string)
		if agentID == "" || apiKey == "" {
			return fmt.Errorf("registration response missing agent_id or api_key")
		}
		client.APIKey = apiKey

		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}
		cfg := &config.Config{
			GatewayEndpoint: endpoint,
			AgentID:         agentID,
			AgentName:       name,
			Owner:           owner,
			TimeoutSecs:     30,
			APIKey:          apiKey,
		}
		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		printSuccess("agent registered and API key saved")
		printKeyValue("Agent ID", agentID)
		printKeyValue("Gateway", endpoint)

		sendHeartbeat := func() error {
			_, err := client.AirportHeartbeat(agentID, status)
			if err == nil {
				fmt.Printf("heartbeat sent: %s\n", time.Now().Format(time.RFC3339))
			}
			return err
		}

		if err := sendHeartbeat(); err != nil {
			return err
		}
		if once {
			return nil
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := sendHeartbeat(); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	connectCmd.Flags().String("name", "", "Agent name")
	connectCmd.Flags().String("owner", "community", "Agent owner or tenant label")
	connectCmd.Flags().String("status", "online", "Heartbeat status")
	connectCmd.Flags().Duration("interval", 30*time.Second, "Heartbeat interval")
	connectCmd.Flags().Bool("once", false, "Send one heartbeat and exit")
	rootCmd.AddCommand(connectCmd)
}
