// License Generator for eyeVesa Pro/Enterprise
//
// Usage:
//   Step 1: Generate a signing keypair (do this once, keep the private key secret)
//     go run cmd/license-gen/main.go --gen-key
//
//   Step 2: Generate a license for a customer
//     go run cmd/license-gen/main.go --customer "Acme Corp" --tier pro --output acme-license.json
//
//   Step 3: Customer uses the license
//     EYEVESA_LICENSE_KEY=/path/to/acme-license.json ./eyevesa-pro
//
// Environment variables:
//   LICENSE_SIGNING_KEY  - Path to the Ed25519 private key PEM file (default: ./license-signing-key.pem)

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

type LicenseClaims struct {
	Tier         string   `json:"tier"`
	MaxAgents    int      `json:"max_agents"`
	MaxResources int      `json:"max_resources"`
	Customer     string   `json:"customer"`
	IssuedAt     string   `json:"issued_at"`
	ExpiresAt    string   `json:"expires_at"`
	Features     []string `json:"features"`
	Signature    string   `json:"signature"`
}

func main() {
	keyPath := os.Getenv("LICENSE_SIGNING_KEY")
	if keyPath == "" {
		fmt.Fprintln(os.Stderr, "ERROR: LICENSE_SIGNING_KEY environment variable is required")
		fmt.Fprintln(os.Stderr, "  Set it to the path of the Ed25519 private key PEM file")
		fmt.Fprintln(os.Stderr, "  Generate one first: go run cmd/license-gen/main.go --gen-key")
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "--gen-key" {
		outPath := getFlag("--output")
		if outPath == "" {
			fmt.Fprintln(os.Stderr, "ERROR: --output flag is required with --gen-key")
			fmt.Fprintln(os.Stderr, "  Example: go run cmd/license-gen/main.go --gen-key --output /secure/path/license-signing-key.pem")
			os.Exit(1)
		}
		generateKey(outPath)
		return
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  Generate signing key:")
		fmt.Println("    go run cmd/license-gen/main.go --gen-key")
		fmt.Println("")
		fmt.Println("  Generate license for customer:")
		fmt.Println("    go run cmd/license-gen/main.go --customer \"Acme Corp\" --tier pro --output license.json")
		fmt.Println("")
		fmt.Println("  Tiers: pro (default), enterprise")
		fmt.Println("  Env:   LICENSE_SIGNING_KEY=/path/to/private-key.pem")
		os.Exit(1)
	}

	customer := getFlag("--customer")
	if customer == "" {
		fmt.Println("Error: --customer is required")
		os.Exit(1)
	}

	tier := getFlag("--tier")
	if tier == "" {
		tier = "pro"
	}

	output := getFlag("--output")
	if output == "" {
		output = fmt.Sprintf("eyevesa-%s-license.json", tier)
	}

	privateKey := loadPrivateKey(keyPath)
	pubKey := privateKey.Public().(ed25519.PublicKey)
	features := getFeatures(tier)

	claims := LicenseClaims{
		Tier:         tier,
		MaxAgents:    getMaxAgents(tier),
		MaxResources: getMaxResources(tier),
		Customer:     customer,
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:    time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339),
		Features:     features,
	}

	payload, _ := json.Marshal(claims)
	sig := ed25519.Sign(privateKey, payload)
	claims.Signature = hex.EncodeToString(sig)

	outputData, _ := json.MarshalIndent(claims, "", "  ")
	if err := os.WriteFile(output, outputData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing license: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ License generated successfully!")
	fmt.Println("  Customer:", customer)
	fmt.Println("  Tier:    ", tier)
	fmt.Println("  Agents:  ", claims.MaxAgents)
	fmt.Println("  Expires: ", claims.ExpiresAt[:10])
	fmt.Println("  File:    ", output)
	fmt.Println("")
	fmt.Println("  Public key (give this to customer to verify):")
	fmt.Println("  ", hex.EncodeToString(pubKey))
	fmt.Println("")
	fmt.Println("  Customer runs with:")
	fmt.Println("    EYEVESA_LICENSE_KEY=" + output + " ./eyevesa-pro")
}

func generateKey(keyPath string) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	pub := priv.Public().(ed25519.PublicKey)

	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: []byte(priv),
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Signing key generated successfully!")
	fmt.Println("  Private key saved to:", keyPath, "(KEEP SECRET!)")
	fmt.Println("  Public key (hex):", hex.EncodeToString(pub))
	fmt.Println("")
	fmt.Println("  IMPORTANT: Set these environment variables for the Pro build:")
	fmt.Println("    EYEVESA_PUBLIC_KEY=" + hex.EncodeToString(pub))
	fmt.Println("    LICENSE_SIGNING_KEY=" + keyPath)
	fmt.Println("")
	fmt.Println("  Store the private key in a secrets manager. Do NOT commit it to version control.")
}

func loadPrivateKey(path string) ed25519.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading private key: %v\n", err)
		fmt.Fprintln(os.Stderr, "Generate one first: go run cmd/license-gen/main.go --gen-key")
		os.Exit(1)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "ED25519 PRIVATE KEY" {
		fmt.Fprintln(os.Stderr, "Invalid private key file")
		os.Exit(1)
	}

	if len(block.Bytes) != ed25519.PrivateKeySize {
		fmt.Fprintln(os.Stderr, "Invalid key size")
		os.Exit(1)
	}

	return ed25519.PrivateKey(block.Bytes)
}

func getFeatures(tier string) []string {
	base := []string{
		"multi_tenant",
		"multi_layer_hitl",
		"slack_notify",
		"pagerduty",
		"sso",
		"llm",
		"anomaly_detection",
		"budget_enforcement",
		"rate_limiting",
		"kubernetes",
		"multi_level_delegation",
		"push_notifications",
	}
	if tier == "enterprise" {
		return append(base,
			"soc2",
			"hipaa",
			"managed_cloud",
			"dedicated_support",
			"multi_region",
			"hsm_integration",
			"custom_policies",
			"custom_adapters",
		)
	}
	return base
}

func getMaxAgents(tier string) int {
	switch tier {
	case "enterprise":
		return 100000
	case "pro":
		return 1000
	default:
		return 5
	}
}

func getMaxResources(tier string) int {
	switch tier {
	case "enterprise":
		return 100000
	case "pro":
		return 10000
	default:
		return 10
	}
}

func getFlag(name string) string {
	for i, arg := range os.Args {
		if arg == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}
