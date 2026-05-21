package health

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

func TestCheckerHealthReportDraining(t *testing.T) {
	var draining atomic.Bool
	draining.Store(true)

	checker := NewChecker(nil, nil, &draining)
	report := checker.Check(context.Background())

	if report.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy when draining, got %s", report.Status)
	}
	if len(report.Components) != 1 || report.Components[0].Name != "server" {
		t.Errorf("expected server component when draining, got %v", report.Components)
	}
}

func TestCheckerHealthReportNoDB(t *testing.T) {
	var draining atomic.Bool
	pe := policy.NewPolicyEngine("", "")
	checker := NewChecker(nil, pe, &draining)

	report := checker.Check(context.Background())

	found := false
	for _, c := range report.Components {
		if c.Name == "postgresql" {
			found = true
			if c.Status == StatusHealthy {
				t.Error("expected postgresql to be unhealthy without DB pool")
			}
		}
	}
	if !found {
		t.Error("expected postgresql component in report")
	}

	foundOpa := false
	for _, c := range report.Components {
		if c.Name == "opa_policy" {
			foundOpa = true
			if c.Status != StatusHealthy {
				t.Errorf("expected OPA to be healthy with local fallback, got %s", c.Status)
			}
		}
	}
	if !foundOpa {
		t.Error("expected opa_policy component in report")
	}
}

func TestCheckerHealthReportWithTimestamp(t *testing.T) {
	var draining atomic.Bool
	pe := policy.NewPolicyEngine("", "")
	checker := NewChecker(nil, pe, &draining)

	report := checker.Check(context.Background())

	if report.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestCheckOPAWithEmbeddedPolicy(t *testing.T) {
	var draining atomic.Bool
	pe := policy.NewPolicyEngine("../../policies", "")
	checker := NewChecker(nil, pe, &draining)

	report := checker.Check(context.Background())

	foundOpa := false
	for _, c := range report.Components {
		if c.Name == "opa_policy" {
			foundOpa = true
			if c.Status != StatusHealthy {
				t.Errorf("expected OPA to be healthy, got %s, error: %s", c.Status, c.Error)
			}
			if c.Latency == "" {
				t.Error("expected latency to be set for healthy OPA")
			}
		}
	}
	if !foundOpa {
		t.Error("expected opa_policy component")
	}
}

func TestNewCheckerDefaults(t *testing.T) {
	var draining atomic.Bool
	checker := NewChecker(nil, nil, &draining)

	if checker.checkTimeout == 0 {
		t.Error("expected non-zero check timeout")
	}
	if checker.draining == nil {
		t.Error("expected draining to be set")
	}
}

func TestComponentStatusString(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}