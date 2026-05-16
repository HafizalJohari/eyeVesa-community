package hitl

import (
	"testing"
)

func TestDetermineEscalationLevel(t *testing.T) {
	tests := []struct {
		name          string
		trustScore    float64
		tool          string
		params        map[string]interface{}
		riskLevel     string
		expectLevel   EscalationLevel
		expectRisk    RiskLevel
	}{
		{
			name: "auto_deny_very_low_trust",
			trustScore: 0.05,
			tool: "read",
			riskLevel: "low",
			expectLevel: LevelAutoDeny,
			expectRisk: RiskCritical,
		},
		{
			name: "auto_allow_high_trust_low_risk",
			trustScore: 0.9,
			tool: "read",
			riskLevel: "low",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "hitl_medium_trust",
			trustScore: 0.3,
			tool: "read",
			riskLevel: "medium",
			expectLevel: LevelHITL,
			expectRisk: RiskMedium,
		},
		{
			name: "escalated_bank_transfer_high_amount",
			trustScore: 0.7,
			tool: "bank_transfer",
			params: map[string]interface{}{"amount": float64(2000)},
			riskLevel: "high",
			expectLevel: LevelEscalated,
			expectRisk: RiskCritical,
		},
		{
			name: "auto_deny_bank_transfer_very_high",
			trustScore: 0.7,
			tool: "bank_transfer",
			params: map[string]interface{}{"amount": float64(6000)},
			riskLevel: "high",
			expectLevel: LevelAutoDeny,
			expectRisk: RiskCritical,
		},
		{
			name: "escalated_schema_change",
			trustScore: 0.8,
			tool: "database_schema_change",
			riskLevel: "high",
			expectLevel: LevelEscalated,
			expectRisk: RiskCritical,
		},
		{
			name: "hitl_k8s_production",
			trustScore: 0.7,
			tool: "k8s_deploy",
			params: map[string]interface{}{"namespace": "production"},
			riskLevel: "medium",
			expectLevel: LevelHITL,
			expectRisk: RiskHigh,
		},
		{
			name: "escalated_low_trust_high_risk",
			trustScore: 0.4,
			tool: "read",
			riskLevel: "high",
			expectLevel: LevelEscalated,
			expectRisk: RiskHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, risk := DetermineEscalationLevel(tt.trustScore, tt.tool, tt.params, tt.riskLevel)
			if level != tt.expectLevel {
				t.Errorf("EscalationLevel = %d, want %d", level, tt.expectLevel)
			}
			if risk != tt.expectRisk {
				t.Errorf("RiskLevel = %s, want %s", risk, tt.expectRisk)
			}
		})
	}
}

func TestRequiredApprovals(t *testing.T) {
	if n := RequiredApprovals(LevelEscalated); n != 2 {
		t.Errorf("LevelEscalated should require 2 approvals, got %d", n)
	}
	if n := RequiredApprovals(LevelHITL); n != 1 {
		t.Errorf("LevelHITL should require 1 approval, got %d", n)
	}
	if n := RequiredApprovals(LevelAutoAllow); n != 0 {
		t.Errorf("LevelAutoAllow should require 0 approvals, got %d", n)
	}
}

func TestTrustDeltaForDecision(t *testing.T) {
	tests := []struct {
		status string
		delta  float64
	}{
		{"approved", 0.01},
		{"rejected", -0.02},
		{"expired", -0.01},
		{"unknown", 0.0},
	}
	for _, tt := range tests {
		if d := TrustDeltaForDecision(tt.status); d != tt.delta {
			t.Errorf("TrustDeltaForDecision(%q) = %f, want %f", tt.status, d, tt.delta)
		}
	}
}