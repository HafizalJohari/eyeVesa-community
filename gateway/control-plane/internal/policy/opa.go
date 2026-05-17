package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Decision struct {
	Allowed             bool    `json:"allowed"`
	RequiresHITL        bool    `json:"requires_hitl"`
	RequiresEscalation  bool    `json:"requires_escalation"`
	Reason              string  `json:"reason"`
	TrustDelta          float64 `json:"trust_delta"`
	EscalationLevel     int     `json:"escalation_level"`
	RequiredApprovals   int     `json:"required_approvals"`
	RiskLevel           string  `json:"risk_level"`
}

type PolicyInput struct {
	Agent struct {
		ID           string   `json:"id"`
		Owner        string   `json:"owner"`
		TrustScore   float64  `json:"trust_score"`
		AllowedTools []string `json:"allowed_tools"`
	} `json:"agent"`
	Action struct {
		Tool          string                 `json:"tool"`
		ResourceID    string                 `json:"resource_id"`
		Params        map[string]interface{} `json:"params"`
		EstimatedCost float64                `json:"estimated_cost"`
	} `json:"action"`
}

type PolicyEngine struct {
	embeddedOPA    *EmbeddedOPA
	opaClient      *OPAClient
	useEmbedded    bool
	useExternal    bool
}

func NewPolicyEngine(policyDir string, opaEndpoint string) *PolicyEngine {
	eng := &PolicyEngine{}

	embedded, err := NewEmbeddedOPA(policyDir)
	if err != nil {
		fmt.Printf("WARN: embedded OPA init failed: %v, will use local fallback\n", err)
	} else {
		eng.embeddedOPA = embedded
		eng.useEmbedded = true
		fmt.Println("INFO: embedded OPA policy engine initialized")
	}

	if opaEndpoint != "" {
		eng.opaClient = NewOPAClient(opaEndpoint)
		eng.useExternal = true
	}

	return eng
}

func (e *PolicyEngine) Evaluate(ctx context.Context, input PolicyInput) *Decision {
	if e.useEmbedded && e.embeddedOPA != nil {
		decision, err := e.embeddedOPA.Evaluate(ctx, input)
		if err == nil {
			return decision
		}
		fmt.Printf("WARN: embedded OPA evaluate failed: %v, falling back\n", err)
	}

	if e.useExternal && e.opaClient != nil {
		decision, err := e.opaClient.Evaluate(ctx, input)
		if err == nil {
			return decision
		}
		fmt.Printf("WARN: external OPA evaluate failed: %v, falling back to local\n", err)
	}

	return LocalEvaluate(input)
}

func (e *PolicyEngine) Reload(policyDir string) error {
	embedded, err := NewEmbeddedOPA(policyDir)
	if err != nil {
		return fmt.Errorf("reload embedded OPA: %w", err)
	}
	e.embeddedOPA = embedded
	e.useEmbedded = true
	return nil
}

type OPAClient struct {
	endpoint string
	client   *http.Client
}

func NewOPAClient(endpoint string) *OPAClient {
	if endpoint == "" {
		endpoint = "http://localhost:8181"
	}
	return &OPAClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *OPAClient) Evaluate(ctx context.Context, input PolicyInput) (*Decision, error) {
	payload := map[string]interface{}{
		"input": input,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v1/data/agentid/authz", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OPA request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPA response: %w", err)
	}

	var opaResp struct {
		Result struct {
			Allow        bool    `json:"allow"`
			RequiresHitl bool    `json:"requires_hitl"`
			Reason       string  `json:"reason"`
			TrustDelta   float64 `json:"trust_delta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &opaResp); err != nil {
		return nil, fmt.Errorf("failed to parse OPA response: %w", err)
	}

	return &Decision{
		Allowed:      opaResp.Result.Allow,
		RequiresHITL: opaResp.Result.RequiresHitl,
		Reason:       opaResp.Result.Reason,
		TrustDelta:   opaResp.Result.TrustDelta,
	}, nil
}

func LocalEvaluate(input PolicyInput) *Decision {
	// Check for auto-deny conditions first
	if input.Agent.TrustScore < 0.1 {
		return &Decision{
			Allowed:            false,
			RequiresHITL:      false,
			RequiresEscalation: false,
			Reason:             "trust score below minimum threshold (0.1)",
			TrustDelta:         -0.05,
			EscalationLevel:    -1,
			RiskLevel:          "critical",
		}
	}

	// Check for auto-deny: bank_transfer > 5000
	if input.Action.Tool == "bank_transfer" {
		if amount, ok := input.Action.Params["amount"].(float64); ok && amount > 5000 {
			return &Decision{
				Allowed:            false,
				RequiresHITL:      false,
				RequiresEscalation: false,
				Reason:             "auto-deny: bank_transfer amount exceeds hard limit ($5000)",
				TrustDelta:         -0.05,
				EscalationLevel:    -1,
				RiskLevel:          "critical",
			}
		}
	}

	// Check tool is in allowed list
	found := false
	for _, tool := range input.Agent.AllowedTools {
		if tool == input.Action.Tool {
			found = true
			break
		}
	}

	if !found {
		return &Decision{
			Allowed:            false,
			RequiresHITL:      true,
			RequiresEscalation: false,
			Reason:             "tool not in agent allowed list",
			TrustDelta:        -0.05,
			EscalationLevel:    1,
			RequiredApprovals:  1,
			RiskLevel:          "medium",
		}
	}

	// Check budget
	if input.Action.EstimatedCost > 0 && input.Action.EstimatedCost > input.Agent.TrustScore*100 {
		return &Decision{
			Allowed:            false,
			RequiresHITL:      false,
			RequiresEscalation: false,
			Reason:             "estimated cost exceeds trust-based budget",
			TrustDelta:        -0.1,
			RiskLevel:          "high",
		}
	}

	// Check for escalation conditions
	if input.Action.Tool == "bank_transfer" {
		if amount, ok := input.Action.Params["amount"].(float64); ok {
			if amount > 1000 {
				return &Decision{
					Allowed:            true,
					RequiresHITL:      true,
					RequiresEscalation: true,
					Reason:             "escalation required: bank_transfer amount > $1000",
					TrustDelta:         0,
					EscalationLevel:    2,
					RequiredApprovals:  2,
					RiskLevel:          "critical",
				}
			}
			if amount > 100 {
				return &Decision{
					Allowed:            true,
					RequiresHITL:      true,
					RequiresEscalation: false,
					Reason:             "HITL required: bank_transfer amount > $100",
					TrustDelta:         0,
					EscalationLevel:    1,
					RequiredApprovals:  1,
					RiskLevel:          "high",
				}
			}
		}
	}

	if input.Action.Tool == "database_schema_change" {
		return &Decision{
			Allowed:            true,
			RequiresHITL:      true,
			RequiresEscalation: true,
			Reason:             "escalation required: database schema changes need 2+ approvals",
			TrustDelta:         0,
			EscalationLevel:    2,
			RequiredApprovals:  2,
			RiskLevel:          "critical",
		}
	}

	if input.Action.Tool == "k8s_deploy" {
		if ns, ok := input.Action.Params["namespace"].(string); ok && ns == "production" {
			return &Decision{
				Allowed:            true,
				RequiresHITL:      true,
				RequiresEscalation: false,
				Reason:             "HITL required: production deployment",
				TrustDelta:         0,
				EscalationLevel:    1,
				RequiredApprovals:  1,
				RiskLevel:          "high",
			}
		}
	}

	// Auto-allow: high trust score + low risk
	if input.Agent.TrustScore >= 0.8 {
		return &Decision{
			Allowed:            true,
			RequiresHITL:      false,
			RequiresEscalation: false,
			Reason:             "auto-allow: high trust score",
			TrustDelta:         0.01,
			RiskLevel:          "low",
		}
	}

	// Default: allow with HITL
	return &Decision{
		Allowed:            true,
		RequiresHITL:      input.Agent.TrustScore < 0.8,
		RequiresEscalation: false,
		Reason:             "allowed: tool in allowed list",
		TrustDelta:         0.01,
		EscalationLevel:    0,
		RequiredApprovals:  0,
		RiskLevel:          "medium",
	}
}