//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

func TestHealthCheck(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected 'ok', got '%s'", string(body))
	}
}

func TestRegisterAndGetAgent(t *testing.T) {
	body := map[string]interface{}{
		"name":           "integration-test-agent",
		"owner":          "test-team",
		"capabilities":   []string{"mcp", "ptv"},
		"allowed_tools":  []string{"read", "write", "search"},
		"max_budget_usd": 100.0,
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post("http://localhost:8080/v1/agents/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	agentID, ok := result["agent_id"].(string)
	if !ok || agentID == "" {
		t.Fatal("expected agent_id in response")
	}

	getResp, err := http.Get(fmt.Sprintf("http://localhost:8080/v1/agents/%s", agentID))
	if err != nil {
		t.Fatalf("get agent failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", getResp.StatusCode)
	}
}

func TestAuthorizeFlow(t *testing.T) {
	body := map[string]interface{}{
		"name":          "auth-test-agent",
		"owner":         "test-team",
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read", "write"},
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post("http://localhost:8080/v1/agents/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer resp.Body.Close()

	var regResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&regResult)
	agentID := regResult["agent_id"].(string)

	tests := []struct {
		name      string
		action    string
		wantAllow bool
		wantHITL  bool
	}{
		{"read should be allowed", "read", true, false},
		{"delete should require HITL", "delete", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authBody, _ := json.Marshal(map[string]interface{}{
				"agent_id":    agentID,
				"action":      tt.action,
				"resource_id": "doc-001",
			})

			authResp, err := http.Post("http://localhost:8080/v1/authorize", "application/json", bytes.NewReader(authBody))
			if err != nil {
				t.Fatalf("authorize failed: %v", err)
			}
			defer authResp.Body.Close()

			var authResult map[string]interface{}
			json.NewDecoder(authResp.Body).Decode(&authResult)

			allowed, _ := authResult["allowed"].(bool)
			requiresHitl, _ := authResult["requires_hitl"].(bool)

			if allowed != tt.wantAllow {
				t.Errorf("allowed = %v, want %v", allowed, tt.wantAllow)
			}
			if requiresHitl != tt.wantHITL {
				t.Errorf("requires_hitl = %v, want %v", requiresHitl, tt.wantHITL)
			}
		})
	}
}

func TestPTVAttestBindVerify(t *testing.T) {
	attestBody, _ := json.Marshal(map[string]interface{}{
		"agent_id":         "ptv-integration-agent",
		"platform":         "linux-tpm2",
		"firmware_version": "2.0.0",
	})

	attestResp, err := http.Post("http://localhost:8080/v1/ptv/attest", "application/json", bytes.NewReader(attestBody))
	if err != nil {
		t.Fatalf("attest failed: %v", err)
	}
	defer attestResp.Body.Close()

	var attestResult map[string]interface{}
	json.NewDecoder(attestResp.Body).Decode(&attestResult)

	if _, ok := attestResult["tpm_signature"]; !ok {
		t.Fatal("expected tpm_signature in attestation")
	}

	bindResp, err := http.Post("http://localhost:8080/v1/ptv/bind", "application/json", bytes.NewReader(attestBody))
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	defer bindResp.Body.Close()

	var bindResult map[string]interface{}
	json.NewDecoder(bindResp.Body).Decode(&bindResult)

	bindingID, ok := bindResult["binding_id"].(string)
	if !ok || bindingID == "" {
		t.Fatal("expected binding_id in bind response")
	}

	verifyResp, err := http.Get(fmt.Sprintf("http://localhost:8080/v1/ptv/verify/%s", bindingID))
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	defer verifyResp.Body.Close()

	var verifyResult map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&verifyResult)

	valid, _ := verifyResult["valid"].(bool)
	if !valid {
		t.Error("expected binding to be valid")
	}
}

func TestHITLWorkflow(t *testing.T) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"agent_id":   "hitl-integration-agent",
		"action":     "bank_transfer",
		"reason":     "Transfer $10K externally",
		"risk_level": "high",
	})

	resp, err := http.Post("http://localhost:8080/v1/hitl/request", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("HITL request failed: %v", err)
	}
	defer resp.Body.Close()

	var reqResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&reqResult)

	approvalID, ok := reqResult["approval_id"].(string)
	if !ok || approvalID == "" {
		t.Fatal("expected approval_id")
	}

	status, _ := reqResult["status"].(string)
	if status != "pending" {
		t.Errorf("expected pending, got %s", status)
	}

	decideBody, _ := json.Marshal(map[string]interface{}{
		"approval_id":     approvalID,
		"approved":        true,
		"approver_method": "faceid",
	})

	decideResp, err := http.Post(fmt.Sprintf("http://localhost:8080/v1/hitl/%s/decide", approvalID), "application/json", bytes.NewReader(decideBody))
	if err != nil {
		t.Fatalf("HITL decide failed: %v", err)
	}
	defer decideResp.Body.Close()

	var decideResult map[string]interface{}
	json.NewDecoder(decideResp.Body).Decode(&decideResult)

	approved, _ := decideResult["approved"].(bool)
	if !approved {
		t.Error("expected approval to be approved")
	}
}

func TestOPAPolicyEngine(t *testing.T) {
	pe := policy.NewPolicyEngine("../../policies", "")

	tests := []struct {
		name      string
		tools     []string
		action    string
		cost      float64
		wantAllow bool
		wantHITL  bool
	}{
		{"allowed tool read", []string{"read", "write"}, "read", 0, true, false},
		{"denied tool delete", []string{"read"}, "delete", 0, false, true},
		{"cost over budget", []string{"write"}, "write", 500, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := policy.PolicyInput{}
			input.Agent.ID = "test-agent"
			input.Agent.TrustScore = 1.0
			input.Agent.AllowedTools = tt.tools
			input.Action.Tool = tt.action
			input.Action.EstimatedCost = tt.cost

			decision := pe.Evaluate(nil, input)

			if decision.Allowed != tt.wantAllow {
				t.Errorf("allowed = %v, want %v", decision.Allowed, tt.wantAllow)
			}
			if decision.RequiresHITL != tt.wantHITL {
				t.Errorf("requires_hitl = %v, want %v", decision.RequiresHITL, tt.wantHITL)
			}
		})
	}
}

func TestMCPProtocol(t *testing.T) {
	mcpBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
	})

	resp, err := http.Post("http://localhost:8080/v1/mcp", "application/json", bytes.NewReader(mcpBody))
	if err != nil {
		t.Fatalf("MCP initialize failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	resultData, _ := result["result"].(map[string]interface{})
	if resultData == nil {
		t.Fatal("expected result in MCP response")
	}

	protoVersion, _ := resultData["protocolVersion"].(string)
	if protoVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", protoVersion)
	}
}