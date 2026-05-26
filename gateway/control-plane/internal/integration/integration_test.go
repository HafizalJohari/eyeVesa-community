//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

const testPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func endpoint(path string) string {
	baseURL := strings.TrimRight(os.Getenv("EYEVESA_INTEGRATION_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return baseURL + path
}

func registerTestAgent(t *testing.T, name string, allowedTools []string) string {
	t.Helper()
	body := map[string]interface{}{
		"name":          name,
		"owner":         "test-team",
		"public_key":    testPublicKey,
		"capabilities":  []string{"mcp"},
		"allowed_tools": allowedTools,
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 registering agent, got %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	agentID, _ := result["agent_id"].(string)
	if agentID == "" {
		t.Fatalf("expected agent_id in response: %v", result)
	}
	return agentID
}

func TestHealthCheck(t *testing.T) {
	resp, err := http.Get(endpoint("/health"))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("expected JSON health response, got %q: %v", string(body), err)
	}
	if health["status"] != "healthy" {
		t.Errorf("expected healthy status, got %v", health["status"])
	}
}

func TestRegisterAndGetAgent(t *testing.T) {
	body := map[string]interface{}{
		"name":           "integration-test-agent",
		"owner":          "test-team",
		"public_key":     testPublicKey,
		"capabilities":   []string{"mcp", "ptv"},
		"allowed_tools":  []string{"read", "write", "search"},
		"max_budget_usd": 100.0,
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(b))
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

	getResp, err := http.Get(fmt.Sprintf(endpoint("/v1/agents/%s"), agentID))
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
		"public_key":    testPublicKey,
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read", "write"},
	}
	b, _ := json.Marshal(body)

	resp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(b))
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

			authResp, err := http.Post(endpoint("/v1/authorize"), "application/json", bytes.NewReader(authBody))
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
	agentID := registerTestAgent(t, "ptv-integration-agent", []string{"read"})
	attestBody, _ := json.Marshal(map[string]interface{}{
		"agent_id":         agentID,
		"platform":         "linux-tpm2",
		"firmware_version": "2.0.0",
	})

	attestResp, err := http.Post(endpoint("/v1/ptv/attest"), "application/json", bytes.NewReader(attestBody))
	if err != nil {
		t.Fatalf("attest failed: %v", err)
	}
	defer attestResp.Body.Close()

	var attestResult map[string]interface{}
	json.NewDecoder(attestResp.Body).Decode(&attestResult)

	if _, ok := attestResult["tpm_signature"]; !ok {
		t.Fatal("expected tpm_signature in attestation")
	}

	bindBody, _ := json.Marshal(attestResult)
	bindResp, err := http.Post(endpoint("/v1/ptv/bind"), "application/json", bytes.NewReader(bindBody))
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

	verifyResp, err := http.Get(fmt.Sprintf(endpoint("/v1/ptv/verify/%s"), bindingID))
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
	agentID := registerTestAgent(t, "hitl-integration-agent", []string{"read", "bank_transfer"})
	reqBody, _ := json.Marshal(map[string]interface{}{
		"agent_id":   agentID,
		"action":     "bank_transfer",
		"reason":     "Transfer $10K externally",
		"risk_level": "high",
	})

	resp, err := http.Post(endpoint("/v1/hitl/request"), "application/json", bytes.NewReader(reqBody))
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

	decideResp, err := http.Post(fmt.Sprintf(endpoint("/v1/hitl/%s/decide"), approvalID), "application/json", bytes.NewReader(decideBody))
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

			decision := pe.Evaluate(context.Background(), input)

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

	resp, err := http.Post(endpoint("/v1/mcp"), "application/json", bytes.NewReader(mcpBody))
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

func TestSkillsCRUD(t *testing.T) {
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":                 "integration-k8s",
		"description":          "Kubernetes deployment skill",
		"category":             "infrastructure",
		"required_proficiency": 3,
		"required_trust_min":   0.5,
	})

	createResp, err := http.Post(endpoint("/v1/skills"), "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("create skill failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated && createResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 201/200, got %d", createResp.StatusCode)
	}

	var createResult map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createResult)

	skillID, ok := createResult["skill_id"].(string)
	if !ok || skillID == "" {
		skillID, _ = createResult["id"].(string)
	}
	if skillID == "" {
		t.Fatal("expected skill_id in create response")
	}

	listResp, err := http.Get(endpoint("/v1/skills"))
	if err != nil {
		t.Fatalf("list skills failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for list, got %d", listResp.StatusCode)
	}
}

func TestSkillsAssignmentAndEndorsement(t *testing.T) {
	agentBody, _ := json.Marshal(map[string]interface{}{
		"name":          "skill-test-agent",
		"owner":         "test-team",
		"public_key":    testPublicKey,
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read", "k8s_deploy"},
	})
	agentResp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(agentBody))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer agentResp.Body.Close()

	var agentResult map[string]interface{}
	json.NewDecoder(agentResp.Body).Decode(&agentResult)
	agentID, _ := agentResult["agent_id"].(string)

	skillBody, _ := json.Marshal(map[string]interface{}{
		"name":                 "endorse-test-skill",
		"description":          "Skill for endorsement testing",
		"category":             "testing",
		"required_proficiency": 2,
		"required_trust_min":   0.3,
	})
	skillResp, err := http.Post(endpoint("/v1/skills"), "application/json", bytes.NewReader(skillBody))
	if err != nil {
		t.Fatalf("create skill failed: %v", err)
	}
	defer skillResp.Body.Close()

	var skillResult map[string]interface{}
	json.NewDecoder(skillResp.Body).Decode(&skillResult)
	skillID, _ := skillResult["skill_id"].(string)
	if skillID == "" {
		skillID, _ = skillResult["id"].(string)
	}

	if skillID != "" && agentID != "" {
		assignBody, _ := json.Marshal(map[string]interface{}{
			"proficiency": 3,
		})
		assignResp, err := http.Post(fmt.Sprintf(endpoint("/v1/skills/%s/assign?agent_id=%s"), skillID, agentID), "application/json", bytes.NewReader(assignBody))
		if err != nil {
			t.Fatalf("assign skill failed: %v", err)
		}
		defer assignResp.Body.Close()

		endorseBody, _ := json.Marshal(map[string]interface{}{
			"endorser_id": agentID,
			"comment":     "Integration test endorsement",
		})
		endorseResp, err := http.Post(fmt.Sprintf(endpoint("/v1/skills/%s/endorse?agent_id=%s"), skillID, agentID), "application/json", bytes.NewReader(endorseBody))
		if err != nil {
			t.Fatalf("endorse skill failed: %v", err)
		}
		defer endorseResp.Body.Close()
	}
}

func TestTransactionProtocol(t *testing.T) {
	agentBody, _ := json.Marshal(map[string]interface{}{
		"name":          "tx-test-agent",
		"owner":         "test-team",
		"public_key":    testPublicKey,
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read", "write", "deploy"},
	})
	agentResp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(agentBody))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer agentResp.Body.Close()

	var agentResult map[string]interface{}
	json.NewDecoder(agentResp.Body).Decode(&agentResult)
	agentID, _ := agentResult["agent_id"].(string)
	if agentID == "" {
		t.Fatal("expected agent_id from registration")
	}

	issueBody, _ := json.Marshal(map[string]interface{}{
		"agent_id":    agentID,
		"resource_id": "res-tx-test",
		"action":      "read",
		"scopes":      []string{"read"},
	})
	issueResp, err := http.Post(endpoint("/v1/tx/issue"), "application/json", bytes.NewReader(issueBody))
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}
	defer issueResp.Body.Close()

	var issueResult map[string]interface{}
	json.NewDecoder(issueResp.Body).Decode(&issueResult)

	allowed, _ := issueResult["allowed"].(bool)
	if !allowed {
		t.Fatalf("expected token issuance to be allowed, got: %v", issueResult)
	}

	tokenData, _ := issueResult["capability_token"].(map[string]interface{})
	if tokenData == nil {
		t.Fatal("expected capability_token in response")
	}

	tokenID, _ := tokenData["jti"].(string)
	if tokenID == "" {
		t.Fatal("expected jti in capability token")
	}

	tokenJSON, _ := json.Marshal(tokenData)
	verifyBody, _ := json.Marshal(map[string]interface{}{"token": string(tokenJSON)})
	verifyResp, err := http.Post(endpoint("/v1/tx/verify"), "application/json", bytes.NewReader(verifyBody))
	if err != nil {
		t.Fatalf("verify token failed: %v", err)
	}
	defer verifyResp.Body.Close()

	var verifyResult map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&verifyResult)

	valid, _ := verifyResult["valid"].(bool)
	if !valid {
		t.Errorf("expected token to be valid, got: %v", verifyResult)
	}

	receiptBody, _ := json.Marshal(map[string]interface{}{"token": string(tokenJSON)})
	receiptResp, err := http.Post(endpoint("/v1/tx/receipt"), "application/json", bytes.NewReader(receiptBody))
	if err != nil {
		t.Fatalf("issue receipt failed: %v", err)
	}
	defer receiptResp.Body.Close()

	var receiptResult map[string]interface{}
	json.NewDecoder(receiptResp.Body).Decode(&receiptResult)

	receiptValid, _ := receiptResult["valid"].(bool)
	if !receiptValid {
		t.Fatalf("expected receipt to be valid, got: %v", receiptResult)
	}

	receiptData, _ := receiptResult["receipt"].(map[string]interface{})
	if receiptData == nil {
		t.Fatal("expected receipt in response")
	}

	receiptJSON, _ := json.Marshal(receiptData)
	receiptVerifyBody, _ := json.Marshal(map[string]interface{}{"receipt": string(receiptJSON)})
	receiptVerifyResp, err := http.Post(endpoint("/v1/tx/receipt/verify"), "application/json", bytes.NewReader(receiptVerifyBody))
	if err != nil {
		t.Fatalf("verify receipt failed: %v", err)
	}
	defer receiptVerifyResp.Body.Close()

	var receiptVerifyResult map[string]interface{}
	json.NewDecoder(receiptVerifyResp.Body).Decode(&receiptVerifyResult)

	rvValid, _ := receiptVerifyResult["valid"].(bool)
	if !rvValid {
		t.Errorf("expected receipt verification to be valid, got: %v", receiptVerifyResult)
	}

	if tokenID != "" {
		revokeBody, _ := json.Marshal(map[string]interface{}{
			"reason": "integration test revocation",
		})
		revokeResp, err := http.Post(fmt.Sprintf(endpoint("/v1/tx/revoke/%s"), tokenID), "application/json", bytes.NewReader(revokeBody))
		if err != nil {
			t.Fatalf("revoke token failed: %v", err)
		}
		defer revokeResp.Body.Close()

		var revokeResult map[string]interface{}
		json.NewDecoder(revokeResp.Body).Decode(&revokeResult)
	}
}

func TestTransactionTokenDenied(t *testing.T) {
	agentBody, _ := json.Marshal(map[string]interface{}{
		"name":          "tx-denied-agent",
		"owner":         "test-team",
		"public_key":    testPublicKey,
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read"},
	})
	agentResp, err := http.Post(endpoint("/v1/agents/register"), "application/json", bytes.NewReader(agentBody))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer agentResp.Body.Close()

	var agentResult map[string]interface{}
	json.NewDecoder(agentResp.Body).Decode(&agentResult)
	agentID, _ := agentResult["agent_id"].(string)

	issueBody, _ := json.Marshal(map[string]interface{}{
		"agent_id": agentID,
		"action":   "nuclear_launch",
	})
	issueResp, err := http.Post(endpoint("/v1/tx/issue"), "application/json", bytes.NewReader(issueBody))
	if err != nil {
		t.Fatalf("issue token request failed: %v", err)
	}
	defer issueResp.Body.Close()

	var issueResult map[string]interface{}
	json.NewDecoder(issueResp.Body).Decode(&issueResult)

	allowed, _ := issueResult["allowed"].(bool)
	if allowed {
		t.Error("nuclear_launch should be denied for basic agent")
	}
}

func TestSPIREIdentity(t *testing.T) {
	listResp, err := http.Get(endpoint("/v1/spire/trust-bundle"))
	if err != nil {
		t.Fatalf("get trust bundle failed: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(listResp.Body).Decode(&result)
		t.Logf("SPIRE trust bundle returned: %v", result)
	} else {
		t.Logf("SPIRE not configured (status %d), skipping SPIRE tests", listResp.StatusCode)
	}
}

func TestSPIREWorkloadRegistration(t *testing.T) {
	regBody, _ := json.Marshal(map[string]interface{}{
		"spiffe_id": "spiffe://example.com/integration-test",
		"selector":  "unix:uid:1000",
		"ttl":       3600,
	})
	regResp, err := http.Post(endpoint("/v1/spire/register"), "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("SPIRE register request failed: %v", err)
	}
	defer regResp.Body.Close()

	if regResp.StatusCode == http.StatusOK || regResp.StatusCode == http.StatusCreated {
		t.Log("SPIRE workload registration succeeded")
	} else {
		t.Logf("SPIRE not available (status %d), workload registration skipped", regResp.StatusCode)
	}
}
