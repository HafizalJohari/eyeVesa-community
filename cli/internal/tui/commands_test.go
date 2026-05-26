package tui

import (
	"strings"
	"testing"
)

func TestProcessCommand(t *testing.T) {
	// Initialize a dummy model with some basic mock state
	m := model{
		agents: []Agent{
			{Name: "hermes-ops", DID: "did:eyevesa:agent:7F3A", Trust: 95, Status: "VERIFIED"},
			{Name: "bad-actor", DID: "did:eyevesa:agent:0000", Trust: 10, Status: "BLOCKED"},
		},
		hitlRequests: []HITLRequest{
			{ID: "hitl-001", AgentID: "hermes-ops", Action: "k8s_deploy", RiskLevel: "HIGH", Status: "PENDING"},
		},
		skills: []Skill{
			{Name: "k8s-deployer", Category: "Infrastructure", RiskLevel: "HIGH", TrustMin: "90%"},
		},
		trustBundles: []TrustBundle{
			{TrustDomain: "spire.eyevesa.local", Type: "SPIFFE", Federated: "YES", Status: "VERIFIED"},
		},
		apiKeys: []APIKeyEntry{
			{ID: "key-1", Name: "ops-key", TenantID: "acme", Status: "ACTIVE", CreatedAt: "2026-05-23"},
		},
		securityEvents: []SecurityEvent{
			{Workflow: "gosec-scan", Conclusion: "success", Branch: "main", RunAt: "2026-05-23"},
		},
	}

	tests := []struct {
		name     string
		command  string
		contains string
	}{
		{
			"Help command",
			"help",
			"Available Commands",
		},
		{
			"List agents",
			"list agents",
			"hermes-ops",
		},
		{
			"Check agent exact",
			"check agent hermes-ops",
			"Trust Score: 95%",
		},
		{
			"Check agent not found",
			"check agent non-existent",
			"not found in registry",
		},
		{
			"HITL list",
			"hitl list",
			"hitl-001",
		},
		{
			"Airport basic (empty)",
			"airport",
			"No agents visible at the airport",
		},
		{
			"Skills list",
			"skills list",
			"k8s-deployer",
		},
		{
			"Skills search found",
			"skills search k8s",
			"k8s-deployer",
		},
		{
			"Skills search not found",
			"skills search nonexistent",
			"No skills matching",
		},
		{
			"Federation list",
			"federation list",
			"spire.eyevesa.local",
		},
		{
			"API keys list",
			"apikeys list",
			"ops-key",
		},
		{
			"Security status",
			"security",
			"gosec-scan",
		},
		{
			"Simulate deploy",
			"simulate deploy_request",
			"Decision: PENDING_APPROVAL",
		},
		{
			"Unknown command",
			"invalidcmd",
			"Unknown command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := ProcessCommand(m, tt.command)
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Expected output of %q to contain %q, but got:\n%s", tt.command, tt.contains, output)
			}
		})
	}
}
