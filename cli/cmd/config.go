package cmd

import (
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/hafizaljohari/eyeVesa/cli/internal/config"
	"github.com/hafizaljohari/eyeVesa/cli/internal/crypto"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage configuration",
	Long:  "View, edit, and manage the eyevesa configuration file.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Println("Configuration:")
		printKeyValue("Config file", cfgPath)
		printKeyValue("Gateway endpoint", cfg.GatewayEndpoint)
		printKeyValue("Agent ID", cfg.AgentID)
		printKeyValue("Agent name", cfg.AgentName)
		printKeyValue("Owner", cfg.Owner)
		printKeyValue("Key path", cfg.KeyPath)
		printKeyValue("Timeout (s)", fmt.Sprintf("%d", cfg.TimeoutSecs))

		if cfg.KeyPath != "" {
			keyPath := cfg.KeyPath
			if keyPath[:2] == "~/" {
				home, _ := os.UserHomeDir()
				keyPath = home + keyPath[1:]
			}
			if _, err := os.Stat(keyPath); err == nil {
				printKeyValue("Keypair", "present")
				priv, err := crypto.LoadPrivateKey(keyPath)
				if err == nil {
					pub := priv.Public().(ed25519.PublicKey)
					printKeyValue("Public key", crypto.PublicKeyToBase64(pub))
				}
			} else {
				printKeyValue("Keypair", "not found")
			}
		}

		return nil
	},
}

var (
	configSetGateway string
	configSetAPIKey  string
	configSetJWT     string
	configSetAgentID string
	configSetName    string
	configSetOwner   string
	configSetKeyPath string
)

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Save gateway, credentials, or agent defaults",
	Long: `Save common eyeVesa CLI settings to ~/.eyevesa/config.toml.

Examples:
  eyevesa config set --gateway http://localhost:8080
  eyevesa config set --api-key eyevesa_xxx
  eyevesa config set --jwt-token eyJ...
  eyevesa config set --agent-id <id> --name my-agent --owner community`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if configSetGateway != "" {
			cfg.GatewayEndpoint = configSetGateway
		}
		if configSetAPIKey != "" {
			cfg.APIKey = configSetAPIKey
		}
		if configSetJWT != "" {
			cfg.JWTToken = configSetJWT
		}
		if configSetAgentID != "" {
			cfg.AgentID = configSetAgentID
		}
		if configSetName != "" {
			cfg.AgentName = configSetName
		}
		if configSetOwner != "" {
			cfg.Owner = configSetOwner
		}
		if configSetKeyPath != "" {
			cfg.KeyPath = configSetKeyPath
		}
		if cfg.TimeoutSecs == 0 {
			cfg.TimeoutSecs = 30
		}

		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		printSuccess("Configuration saved")
		printKeyValue("Config file", cfgPath)
		if cfg.GatewayEndpoint != "" {
			printKeyValue("Gateway endpoint", cfg.GatewayEndpoint)
		}
		if cfg.AgentID != "" {
			printKeyValue("Agent ID", cfg.AgentID)
		}
		if cfg.APIKey != "" {
			printKeyValue("API key", "present")
		}
		if cfg.JWTToken != "" {
			printKeyValue("JWT token", "present")
		}
		return nil
	},
}

func init() {
	configSetCmd.Flags().StringVar(&configSetGateway, "gateway", "", "Gateway endpoint, for example http://localhost:8080")
	configSetCmd.Flags().StringVar(&configSetAPIKey, "api-key", "", "API key used as X-API-Key")
	configSetCmd.Flags().StringVar(&configSetJWT, "jwt-token", "", "JWT token used as Authorization bearer token")
	configSetCmd.Flags().StringVar(&configSetAgentID, "agent-id", "", "Default agent ID")
	configSetCmd.Flags().StringVar(&configSetName, "name", "", "Default agent name")
	configSetCmd.Flags().StringVar(&configSetOwner, "owner", "", "Default owner/tenant label")
	configSetCmd.Flags().StringVar(&configSetKeyPath, "key-path", "", "Path to the agent private key")

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	addStartCommand(configCmd)
}
