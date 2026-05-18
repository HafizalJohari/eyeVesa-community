package cmd

import (
	"fmt"
	"os"

	"github.com/hafizaljohari/eyeVesa/cli/internal/config"
	"github.com/hafizaljohari/eyeVesa/cli/internal/crypto"
	"github.com/spf13/cobra"
)

var (
	initName            string
	initOwner           string
	initCapabilities    []string
	initAllowedTools    []string
	initMaxBudget       float64
	initDelegationPolicy string
	initBehavioralTags  []string
	initGateway         string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Register a new agent and save configuration",
	Long: `Register a new agent with the AgentID Gateway, generate an Ed25519
keypair, and save the configuration to ~/.eyevesa/.

Examples:
  eyevesa init --name hermes-ops --owner org:devops
  eyevesa init --name my-agent --owner org:eng --capabilities "read,write" --allowed-tools "k8s_deploy,log_search" --max-budget 500`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Agent name (required)")
	initCmd.Flags().StringVarP(&initOwner, "owner", "", "", "Agent owner (required)")
	initCmd.Flags().StringSliceVar(&initCapabilities, "capabilities", []string{}, "Agent capabilities (comma-separated)")
	initCmd.Flags().StringSliceVar(&initAllowedTools, "allowed-tools", []string{}, "Allowed tools (comma-separated)")
	initCmd.Flags().Float64Var(&initMaxBudget, "max-budget", 0, "Maximum budget in USD")
	initCmd.Flags().StringVar(&initDelegationPolicy, "delegation-policy", "no_chain", "Delegation policy: no_chain, single_level")
	initCmd.Flags().StringSliceVar(&initBehavioralTags, "behavioral-tags", []string{}, "Behavioral tags (comma-separated)")
	initCmd.Flags().StringVarP(&initGateway, "gateway", "g", "", "Gateway endpoint URL")

	_ = initCmd.MarkFlagRequired("name")
	_ = initCmd.MarkFlagRequired("owner")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("create config dirs: %w", err)
	}

	client := getClient()

	if initGateway != "" {
		client.BaseURL = initGateway
	}

	result, err := client.RegisterAgent(
		initName,
		initOwner,
		initCapabilities,
		initAllowedTools,
		initMaxBudget,
		initDelegationPolicy,
		initBehavioralTags,
	)
	if err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	agentID, _ := result["agent_id"].(string)
	publicKeyB64, _ := result["public_key"].(string)
	status, _ := result["status"].(string)
	trustScore := result["trust_score"]

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generate keypair: %w", err)
	}

	keysDir := config.DefaultKeysDir()
	keyPath := keysDir + "/" + initName + ".key"
	if err := crypto.SavePrivateKey(keyPair, keyPath); err != nil {
		return fmt.Errorf("save private key: %w", err)
	}

	cfgPath := config.DefaultConfigPath()
	cfg := &config.Config{
		GatewayEndpoint: client.BaseURL,
		AgentID:         agentID,
		AgentName:       initName,
		Owner:           initOwner,
		KeyPath:         keyPath,
		TimeoutSecs:    30,
	}
	if err := cfg.Save(cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	printSuccess(fmt.Sprintf("Agent registered: %s", initName))
	printKeyValue("Agent ID", agentID)
	printKeyValue("Public key", publicKeyB64)
	printKeyValue("Status", status)
	fmt.Printf("  %-20s %.4f\n", "Trust score:", trustScore)
	printSuccess(fmt.Sprintf("Keypair saved to %s", keyPath))
	printSuccess(fmt.Sprintf("Config saved to %s", cfgPath))

	health, err := client.Health()
	if err == nil && (health == "ok" || health == "healthy") {
		printSuccess("Gateway connection verified")
	} else {
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Could not verify gateway connection: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  ⚠ Could not verify gateway connection: got %s\n", health)
		}
	}

	return nil
}