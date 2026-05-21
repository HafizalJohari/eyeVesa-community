package policy

import (
	"context"
	"testing"
)

func TestEmbeddedOPAEvaluate(t *testing.T) {
	eopa, err := NewEmbeddedOPA("../../policies")
	if err != nil {
		t.Fatalf("Failed to create embedded OPA: %v", err)
	}

	ctx := context.Background()

	// Test 1: tool in allowed list, should be allowed
	input := PolicyInput{}
	input.Agent.ID = "test-agent"
	input.Agent.Owner = "test-owner"
	input.Agent.TrustScore = 1.0
	input.Agent.AllowedTools = []string{"read", "write"}
	input.Action.Tool = "read"

	decision, err := eopa.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !decision.Allowed {
		t.Errorf("Expected allowed=true, got false")
	}
	if decision.RequiresHITL {
		t.Errorf("Expected requires_hitl=false, got true")
	}
	if decision.Reason != "tool in allowed list" {
		t.Errorf("Expected reason='tool in allowed list', got '%s'", decision.Reason)
	}
	if decision.TrustDelta != 0.01 {
		t.Errorf("Expected trust_delta=0.01, got %f", decision.TrustDelta)
	}

	// Test 2: tool NOT in allowed list, should require HITL
	input2 := PolicyInput{}
	input2.Agent.ID = "test-agent"
	input2.Agent.TrustScore = 1.0
	input2.Agent.AllowedTools = []string{"read"}
	input2.Action.Tool = "delete"

	decision2, err := eopa.Evaluate(ctx, input2)
	if err != nil {
		t.Fatalf("Evaluate2 failed: %v", err)
	}

	if decision2.Allowed {
		t.Errorf("Expected allowed=false, got true")
	}
	if !decision2.RequiresHITL {
		t.Errorf("Expected requires_hitl=true, got false")
	}
	if decision2.Reason != "tool not in agent allowed list" {
		t.Errorf("Expected reason='tool not in agent allowed list', got '%s'", decision2.Reason)
	}

	// Test 3: tool in list but cost exceeds budget
	input3 := PolicyInput{}
	input3.Agent.ID = "test-agent"
	input3.Agent.TrustScore = 0.5
	input3.Agent.AllowedTools = []string{"read", "write"}
	input3.Action.Tool = "write"
	input3.Action.EstimatedCost = 200

	decision3, err := eopa.Evaluate(ctx, input3)
	if err != nil {
		t.Fatalf("Evaluate3 failed: %v", err)
	}

	if decision3.Allowed {
		t.Errorf("Expected allowed=false (cost exceeds budget), got true")
	}
	if !decision3.RequiresHITL {
		t.Errorf("Expected requires_hitl=true (cost exceeds budget), got false")
	}
	if decision3.Reason != "estimated cost exceeds trust budget" {
		t.Errorf("Expected reason='estimated cost exceeds trust budget', got '%s'", decision3.Reason)
	}
	if decision3.TrustDelta != -0.1 {
		t.Errorf("Expected trust_delta=-0.1, got %f", decision3.TrustDelta)
	}

	t.Logf("Test 1: allowed=%v, hitl=%v, reason=%s, delta=%f", decision.Allowed, decision.RequiresHITL, decision.Reason, decision.TrustDelta)
	t.Logf("Test 2: allowed=%v, hitl=%v, reason=%s, delta=%f", decision2.Allowed, decision2.RequiresHITL, decision2.Reason, decision2.TrustDelta)
	t.Logf("Test 3: allowed=%v, hitl=%v, reason=%s, delta=%f", decision3.Allowed, decision3.RequiresHITL, decision3.Reason, decision3.TrustDelta)
}

func TestPolicyEngineReload(t *testing.T) {
	eng := NewPolicyEngine("../../policies", "")
	if eng.embeddedOPA == nil {
		t.Fatal("embedded OPA should be initialized")
	}

	err := eng.Reload("../../policies")
	if err != nil {
		t.Fatalf("Reload should succeed: %v", err)
	}

	err = eng.Reload("/nonexistent/path")
	if err == nil {
		t.Error("Reload with nonexistent path should fail")
	}
}
