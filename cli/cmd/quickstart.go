package cmd

import (
	"fmt"

	"github.com/hafizaljohari/eyeVesa/cli/internal/config"
	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Show the shortest safe path for a first-time user",
	Long: `Show the recommended first steps for using eyeVesa.

This command does not change anything. It explains which command to run next
based on the current CLI configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			cfg = &config.Config{GatewayEndpoint: "http://localhost:8080", TimeoutSecs: 30}
		}
		gateway := gatewayAddr
		if gateway == "" {
			gateway = cfg.GatewayEndpoint
		}
		if gateway == "" {
			gateway = "http://localhost:8080"
		}

		fmt.Println("eyeVesa quickstart")
		fmt.Println()
		fmt.Println("1. Start the local stack")
		fmt.Println("   ./start.sh")
		fmt.Println()
		fmt.Println("2. Check the CLI can reach the gateway")
		fmt.Println("   eyevesa doctor")
		fmt.Println()

		if cfg.APIKey == "" && cfg.JWTToken == "" {
			fmt.Println("3. Save your credential")
			fmt.Println("   eyevesa config set --gateway " + gateway + " --api-key <eyevesa_api_key>")
			fmt.Println()
			fmt.Println("   Production note: API-key creation is an admin action. Ask an admin for a key,")
			fmt.Println("   or use an admin JWT/API key to run:")
			fmt.Println("   eyevesa api-keys create --name my-agent-key")
		} else {
			fmt.Println("3. Credential is configured")
			if cfg.APIKey != "" {
				fmt.Println("   API key: present")
			}
			if cfg.JWTToken != "" {
				fmt.Println("   JWT token: present")
			}
		}
		fmt.Println()

		if cfg.AgentID == "" {
			fmt.Println("4. Register your first agent and send one Airport heartbeat")
			fmt.Println("   eyevesa connect --name my-agent --owner community --once")
		} else {
			fmt.Println("4. Agent is configured")
			fmt.Println("   Agent ID: " + cfg.AgentID)
			fmt.Println("   Send heartbeat: eyevesa airport heartbeat " + cfg.AgentID + " --status online")
		}
		fmt.Println()
		fmt.Println("5. See who is discoverable")
		fmt.Println("   eyevesa airport online")
		fmt.Println("   eyevesa airport search --status online")
		fmt.Println()
		fmt.Println("Use 'eyevesa tui' when you want the guided dashboard.")
		return nil
	},
}

func init() {
	addStartCommand(quickstartCmd)
}
