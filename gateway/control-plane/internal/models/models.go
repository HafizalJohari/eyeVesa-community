package models

import "time"

type Agent struct {
	AgentID          string    `json:"agent_id"`
	Name             string    `json:"name"`
	Owner            string    `json:"owner"`
	PublicKey        []byte    `json:"public_key"`
	Capabilities     []string  `json:"capabilities"`
	AllowedTools     []string  `json:"allowed_tools"`
	MaxBudgetUSD     float64   `json:"max_budget_usd"`
	DelegationPolicy string    `json:"delegation_policy"`
	BehavioralTags   []string  `json:"behavioral_tags"`
	TrustScore       float64   `json:"trust_score"`
	Status           string    `json:"status"`
	TenantID         string    `json:"tenant_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Resource struct {
	ResourceID        string    `json:"resource_id"`
	Name              string    `json:"name"`
	ResourceType      string    `json:"resource_type"`
	Endpoint          string    `json:"endpoint"`
	AuthMethod        string    `json:"auth_method"`
	Capabilities      string    `json:"capabilities"`
	RiskLevel         string    `json:"risk_level"`
	DataSensitivity   string    `json:"data_sensitivity"`
	RateLimitPerAgent int       `json:"rate_limit_per_agent"`
	Status            string    `json:"status"`
	TenantID          string    `json:"tenant_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}