package hitl

import (
	"context"
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
		{
			name: "auto_allow_medium_trust_empty_risk",
			trustScore: 0.6,
			tool: "read",
			riskLevel: "",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "hitl_restricted_low_trust",
			trustScore: 0.5,
			tool: "read",
			riskLevel: "restricted",
			expectLevel: LevelHITL,
			expectRisk: RiskHigh,
		},
		{
			name: "auto_allow_high_trust_restricted",
			trustScore: 0.9,
			tool: "read",
			riskLevel: "restricted",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "bank_transfer_small_amount_hitl",
			trustScore: 0.7,
			tool: "bank_transfer",
			params: map[string]interface{}{"amount": float64(50)},
			riskLevel: "low",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "bank_transfer_medium_amount_hitl",
			trustScore: 0.7,
			tool: "bank_transfer",
			params: map[string]interface{}{"amount": float64(500)},
			riskLevel: "medium",
			expectLevel: LevelHITL,
			expectRisk: RiskHigh,
		},
		{
			name: "bank_transfer_no_amount",
			trustScore: 0.8,
			tool: "bank_transfer",
			params: nil,
			riskLevel: "low",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "k8s_non_production",
			trustScore: 0.7,
			tool: "k8s_deploy",
			params: map[string]interface{}{"namespace": "staging"},
			riskLevel: "medium",
			expectLevel: LevelAutoAllow,
			expectRisk: RiskLow,
		},
		{
			name: "zero_trust",
			trustScore: 0.0,
			tool: "read",
			riskLevel: "low",
			expectLevel: LevelAutoDeny,
			expectRisk: RiskCritical,
		},
		{
			name: "boundary_trust_0_1",
			trustScore: 0.1,
			tool: "read",
			riskLevel: "low",
			expectLevel: LevelHITL,
			expectRisk: RiskMedium,
		},
		{
			name: "schema_change_any_trust",
			trustScore: 0.99,
			tool: "database_schema_change",
			riskLevel: "low",
			expectLevel: LevelEscalated,
			expectRisk: RiskCritical,
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
	if n := RequiredApprovals(LevelAutoDeny); n != 0 {
		t.Errorf("LevelAutoDeny should require 0 approvals, got %d", n)
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

func TestEscalationLevelConstants(t *testing.T) {
	if LevelAutoDeny >= LevelAutoAllow {
		t.Error("AutoDeny should be less than AutoAllow")
	}
	if LevelHITL <= LevelAutoAllow {
		t.Error("HITL should be greater than AutoAllow")
	}
	if LevelEscalated <= LevelHITL {
		t.Error("Escalated should be greater than HITL")
	}
}

func TestRiskLevelConstants(t *testing.T) {
	if RiskLow == "" {
		t.Error("RiskLow should not be empty")
	}
	if RiskCritical == "" {
		t.Error("RiskCritical should not be empty")
	}
}

func TestNotificationChannelConstants(t *testing.T) {
	channels := []NotificationChannel{
		ChannelWebhook, ChannelSlack, ChannelTelegram,
		ChannelDiscord, ChannelEmail, ChannelPagerduty, ChannelPush,
	}
	for _, ch := range channels {
		if string(ch) == "" {
			t.Errorf("channel constant should not be empty")
		}
	}
}

func TestNewEscalationService(t *testing.T) {
	svc := NewEscalationService(nil)
	if svc == nil {
		t.Fatal("NewEscalationService returned nil")
	}
	if svc.notifyChans == nil {
		t.Fatal("notifyChans should be initialized")
	}
}

func TestRegisterNotifier(t *testing.T) {
	svc := NewEscalationService(nil)
	mock := &mockNotifier{}
	svc.RegisterNotifier(ChannelWebhook, mock)
	if _, ok := svc.notifyChans[ChannelWebhook]; !ok {
		t.Fatal("webhook notifier should be registered")
	}
}

func TestEscalationConfig_Fields(t *testing.T) {
	cfg := EscalationConfig{
		ConfigID:       "cfg-1",
		TenantID:       "t-1",
		Level:          1,
		TimeoutSeconds: 300,
		NotifyChannel:  ChannelSlack,
		NotifyTarget:   "#alerts",
	}
	if cfg.ConfigID != "cfg-1" {
		t.Fatalf("ConfigID mismatch: got %s", cfg.ConfigID)
	}
	if cfg.Level != 1 {
		t.Fatalf("Level mismatch: got %d", cfg.Level)
	}
}

func TestApprovalRequest_Fields(t *testing.T) {
	req := ApprovalRequest{
		AgentID:    "a1",
		ResourceID: "r1",
		Action:     "deploy",
		Reason:     "test",
		RiskLevel:  "high",
	}
	if req.AgentID != "a1" {
		t.Fatalf("AgentID mismatch: got %s", req.AgentID)
	}
}

func TestApprovalResponse_Fields(t *testing.T) {
	resp := ApprovalResponse{
		ApprovalID: "ap1",
		AgentID:    "a1",
		Action:     "deploy",
		Status:     "pending",
		ExpiresAt:  "2026-01-01T00:00:00Z",
	}
	if resp.Status != "pending" {
		t.Fatalf("Status mismatch: got %s", resp.Status)
	}
}

func TestApprovalDecision_Fields(t *testing.T) {
	d := ApprovalDecision{
		ApprovalID:     "ap1",
		Approved:       true,
		ApproverMethod: "web",
	}
	if !d.Approved {
		t.Fatal("Approved should be true")
	}
}

func TestApprovalChainEntry_Fields(t *testing.T) {
	e := ApprovalChainEntry{
		ChainID:       "c1",
		ApprovalID:   "ap1",
		ApproverID:   "u1",
		ApprovalLevel: 1,
		Decision:     "approved",
	}
	if e.ApprovalLevel != 1 {
		t.Fatalf("ApprovalLevel mismatch: got %d", e.ApprovalLevel)
	}
}

func TestNotificationEntry_Fields(t *testing.T) {
	e := NotificationEntry{
		NotificationID: "n1",
		ApprovalID:    "ap1",
		Channel:       "slack",
		RecipientID:   "u1",
		EscalationLevel: 1,
	}
	if e.NotificationID != "n1" {
		t.Fatalf("NotificationID mismatch: got %s", e.NotificationID)
	}
}

type mockNotifier struct {
	lastTarget  string
	lastMessage string
	sendErr     error
}

func (m *mockNotifier) Send(ctx context.Context, target string, message string) error {
	m.lastTarget = target
	m.lastMessage = message
	return m.sendErr
}