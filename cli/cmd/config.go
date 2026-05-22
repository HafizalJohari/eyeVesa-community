package cmd

import (
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/HafizalJohari/eyeVesa-community/cli/internal/config"
	"github.com/HafizalJohari/eyeVesa-community/cli/internal/crypto"
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

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
