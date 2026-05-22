package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/HafizalJohari/eyeVesa-community/cli/internal/api"
	"github.com/HafizalJohari/eyeVesa-community/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	gatewayAddr string
	outputFmt   string
	version     = "dev"
)

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("eyevesa {{.Version}}\n")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default ~/.eyevesa/config.toml)")
	rootCmd.PersistentFlags().StringVarP(&gatewayAddr, "gateway", "g", "", "gateway endpoint (default http://localhost:8080)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "text", "output format: text, json")
}

var rootCmd = &cobra.Command{
	Use:   "eyevesa",
	Short: "eyeVesa CLI - identity and trust layer for AI agents",
	Long: `eyevesa is the CLI for the eyeVesa Gateway.

It provides commands to register agents, manage resources, authorize
actions, inspect trust scores, manage skills and endorsements, handle
multi-tenant isolation, approve HITL requests, enforce budget limits,
manage SPIRE identities, and audit activity - all with cryptographic
identity and non-repudiable logs.`,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func getClient() *api.Client {
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}

	appCfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
		appCfg = &config.Config{
			GatewayEndpoint: "http://localhost:8080",
			TimeoutSecs:     30,
		}
	}

	addr := gatewayAddr
	if addr == "" {
		addr = appCfg.GatewayEndpoint
	}
	if addr == "" {
		addr = "http://localhost:8080"
	}

	client := api.NewClient(addr)
	client.APIKey = appCfg.APIKey
	client.JWTToken = appCfg.JWTToken
	return client
}

func printResult(result map[string]interface{}) {
	switch outputFmt {
	case "json":
		data, _ := jsonMarshal(result)
		fmt.Println(string(data))
	default:
		printFlat(result, 0)
	}
}

func printFlat(m map[string]interface{}, indent int) {
	for k, v := range m {
		prefix := strings.Repeat("  ", indent)
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", prefix, k)
			printFlat(val, indent+1)
		case []interface{}:
			fmt.Printf("%s%s:\n", prefix, k)
			for _, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					printFlat(m, indent+1)
					fmt.Println()
				} else {
					fmt.Printf("%s  - %v\n", prefix, item)
				}
			}
		default:
			fmt.Printf("%s%s: %v\n", prefix, k, v)
		}
	}
}

func jsonMarshal(v interface{}) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString("{\n")
	jsonMarshalValue(v, &buf, 1)
	buf.WriteString("\n}")
	return []byte(buf.String()), nil
}

func jsonMarshalValue(v interface{}, buf *strings.Builder, indent int) {
	switch val := v.(type) {
	case map[string]interface{}:
		i := 0
		for k, v := range val {
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString(strings.Repeat("  ", indent))
			buf.WriteString(fmt.Sprintf("%q: ", k))
			jsonMarshalValue(v, buf, indent)
			i++
		}
	case []interface{}:
		buf.WriteString("[\n")
		for i, item := range val {
			buf.WriteString(strings.Repeat("  ", indent+1))
			jsonMarshalValue(item, buf, indent+1)
			if i < len(val)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(strings.Repeat("  ", indent) + "]")
	case string:
		buf.WriteString(fmt.Sprintf("%q", val))
	case float64:
		buf.WriteString(fmt.Sprintf("%v", val))
	case bool:
		buf.WriteString(fmt.Sprintf("%v", val))
	case nil:
		buf.WriteString("null")
	default:
		buf.WriteString(fmt.Sprintf("%q", fmt.Sprintf("%v", val)))
	}
}

func printSuccess(msg string) {
	fmt.Printf("✓ %s\n", msg)
}

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
}

func printKeyValue(key, value string) {
	fmt.Printf("  %-20s %s\n", key+":", value)
}
