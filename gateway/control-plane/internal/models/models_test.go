package models

import (
	"testing"
	"time"
)

func TestAgent_Fields(t *testing.T) {
	a := Agent{
		AgentID:          "a1",
		Name:            "Test Agent",
		Owner:           "owner-1",
		PublicKey:        []byte("key"),
		Capabilities:     []string{"read", "write"},
		AllowedTools:     []string{"deploy"},
		MaxBudgetUSD:     100.0,
		DelegationPolicy: "strict",
		BehavioralTags:   []string{"cautious"},
		TrustScore:       0.85,
		Status:          "active",
		TenantID:        "t1",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if a.AgentID != "a1" {
		t.Fatalf("AgentID mismatch: got %s", a.AgentID)
	}
	if a.TrustScore != 0.85 {
		t.Fatalf("TrustScore mismatch: got %f", a.TrustScore)
	}
	if len(a.Capabilities) != 2 {
		t.Fatalf("Capabilities length mismatch: got %d", len(a.Capabilities))
	}
	if a.MaxBudgetUSD != 100.0 {
		t.Fatalf("MaxBudgetUSD mismatch: got %f", a.MaxBudgetUSD)
	}
}

func TestResource_Fields(t *testing.T) {
	r := Resource{
		ResourceID:        "r1",
		Name:             "Test Resource",
		ResourceType:      "database",
		Endpoint:         "db.example.com:5432",
		AuthMethod:       "mTLS",
		RiskLevel:        "high",
		DataSensitivity:  "confidential",
		RateLimitPerAgent: 100,
		Status:          "active",
		TenantID:        "t1",
		RequiredSkills:   []string{"kubernetes", "postgres"},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if r.ResourceID != "r1" {
		t.Fatalf("ResourceID mismatch: got %s", r.ResourceID)
	}
	if r.RiskLevel != "high" {
		t.Fatalf("RiskLevel mismatch: got %s", r.RiskLevel)
	}
	if len(r.RequiredSkills) != 2 {
		t.Fatalf("RequiredSkills length mismatch: got %d", len(r.RequiredSkills))
	}
	if r.RateLimitPerAgent != 100 {
		t.Fatalf("RateLimitPerAgent mismatch: got %d", r.RateLimitPerAgent)
	}
}

func TestAgent_EmptyFields(t *testing.T) {
	a := Agent{}
	if a.AgentID != "" {
		t.Fatal("empty agent should have empty AgentID")
	}
	if a.TrustScore != 0 {
		t.Fatal("empty agent should have zero TrustScore")
	}
	if a.Capabilities != nil {
		t.Fatal("empty agent should have nil Capabilities")
	}
}

func TestResource_EmptyFields(t *testing.T) {
	r := Resource{}
	if r.ResourceID != "" {
		t.Fatal("empty resource should have empty ResourceID")
	}
}

func TestAgent_RequiredSkills(t *testing.T) {
	a := Agent{AllowedTools: []string{"deploy", "read", "write"}}
	if len(a.AllowedTools) != 3 {
		t.Fatalf("AllowedTools length mismatch: got %d", len(a.AllowedTools))
	}
}

func TestResource_RequiredSkillsEmpty(t *testing.T) {
	r := Resource{}
	if r.RequiredSkills != nil {
		t.Fatal("empty resource should have nil RequiredSkills")
	}
}