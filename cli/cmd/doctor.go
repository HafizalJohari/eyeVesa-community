package cmd

import (
	"fmt"
	"os"

	"github.com/hafizaljohari/eyeVesa/cli/internal/config"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration and connectivity",
	Long: `Check that the eyevesa CLI is properly configured and can reach
the gateway, database, and policy engine.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("eyevesa doctor")
		fmt.Println()

		ok := true

		// Check config
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}
		fmt.Printf("  Config file:     %s ", cfgPath)
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Println("✓")
		} else {
			fmt.Println("✗ (not found, will use defaults)")
		}

		// Check keypair
		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Printf("  Config load:     ✗ (%v)\n", err)
			ok = false
		} else {
			fmt.Printf("  Config load:     ✓\n")
			fmt.Printf("  Gateway:         %s\n", cfg.GatewayEndpoint)
			fmt.Printf("  Agent ID:        %s ", cfg.AgentID)
			if cfg.AgentID != "" {
				fmt.Println("✓")
			} else {
				fmt.Println("(not set, run 'eyevesa init')")
			}

			if cfg.KeyPath != "" {
				fmt.Printf("  Keypair:         %s ", cfg.KeyPath)
				if _, err := os.Stat(cfg.KeyPath); err == nil {
					fmt.Println("✓")
				} else {
					fmt.Println("✗ (key file missing)")
					ok = false
				}
			}
		}

		// Check gateway connectivity
		client := getClient()
		fmt.Printf("  Gateway health:  ")
		health, err := client.Health()
		if err != nil {
			fmt.Printf("✗ (%v)\n", err)
			ok = false
		} else {
			fmt.Printf("✓ (%s)\n", health)
		}

		// Check identity
		fmt.Printf("  Identity:        ")
		idResult, err := client.Identity()
		if err != nil {
			fmt.Printf("✗ (%v)\n", err)
		} else {
			spiffeID, _ := idResult["spiffe_id"].(string)
			fmt.Printf("✓ (%s)\n", spiffeID)
		}

		fmt.Println()
		if ok {
			printSuccess("All checks passed")
		} else {
			printError("Some checks failed. Run 'eyevesa init' to configure.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}