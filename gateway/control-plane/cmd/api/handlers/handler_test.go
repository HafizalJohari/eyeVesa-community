package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

type mockRow struct {
	scanErr error
	vals    []interface{}
}

func (r *mockRow) Scan(dest ...interface{}) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, v := range r.vals {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				*d = v.(string)
			case *float64:
				*d = v.(float64)
			case *int:
				*d = v.(int)
			case *[]byte:
				*d = v.([]byte)
			case *[]string:
				if sv, ok := v.([]string); ok {
					*d = sv
				}
			case *time.Time:
				*d = v.(time.Time)
			}
		}
	}
	return nil
}

type mockRows struct {
	results [][]interface{}
	idx     int
	closed  bool
}

func (r *mockRows) Next() bool {
	r.idx++
	return r.idx <= len(r.results)
}

func (r *mockRows) Scan(dest ...interface{}) error {
	row := r.results[r.idx-1]
	for i, v := range row {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				*d = v.(string)
			case *float64:
				*d = v.(float64)
			}
		}
	}
	return nil
}

func (r *mockRows) Close() {
	r.closed = true
}

type mockQuerier struct {
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) database.Row
	queryFn    func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error)
}

func (m *mockQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) database.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

func (m *mockQuerier) Query(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockQuerier) Exec(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return database.CommandTag{RowsAffected: 1}, nil
}

func setupTestRouter(q *mockQuerier) http.Handler {
	if q == nil {
		q = &mockQuerier{}
	}
	SetQuerier(q)
	SetAuditLogger(nil)
	SetGatewayKeys(nil)
	SetPolicyEngine(policy.NewPolicyEngine("", ""))
	SetDelegationTracker(nil)
	SetHITLService(nil)
	SetPTVService(nil)
	SetEscalationService(nil)
	SetLLMService(nil)
	SetEmbeddingService(nil)
	SetTenantService(nil)
	SetPushService(nil)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/agents/register", RegisterAgent)
		r.Post("/mcp", HandleMCP)
		r.Post("/authorize", Authorize)
		r.Post("/verify-signature", VerifySignature)
		r.Post("/hitl/request", RequestApproval)
		r.Get("/hitl/pending", ListPendingApprovals)
		r.Get("/hitl/{approvalID}", GetApprovalStatus)
		r.Post("/hitl/{approvalID}/decide", DecideApproval)
		r.Post("/delegate", DelegateAgent)
		r.Get("/delegations/{agentID}", GetDelegationChain)
		r.Get("/delegations/validate", ValidateDelegation)
		r.Delete("/delegations/{delegationID}", RevokeDelegation)
		r.Post("/ptv/attest", AttestIdentity)
		r.Post("/ptv/bind", BindIdentity)
		r.Get("/ptv/verify/{bindingID}", VerifyIdentity)
		r.Post("/resources/register", RegisterResource)
		r.Get("/resources", ListResources)
		r.Get("/resources/{resourceID}", GetResource)
		r.Get("/agents", ListAgents)
		r.Get("/agents/{agentID}", GetAgent)
		r.Get("/a2a/agents", ListA2AAgents)
		r.Post("/a2a/tasks", CreateA2ATask)
		r.Get("/a2a/tasks/{taskID}", GetA2ATask)
		r.Post("/hitl/escalate", RequestEscalatedApproval)
		r.Post("/hitl/{approvalID}/chain", ProcessChainDecision)
		r.Get("/hitl/{approvalID}/chain", GetApprovalChain)
		r.Get("/hitl/{approvalID}/notifications", GetNotifications)
		r.Post("/llm/hitl-summary/{approvalID}", GenerateHITLSummary)
		r.Post("/llm/audit-narrative", GenerateAuditNarrative)
		r.Post("/llm/policy-translate", TranslatePolicy)
		r.Post("/behavior/{agentID}/embedding", UpdateBehaviorEmbedding)
		r.Get("/behavior/{agentID}/anomalies", DetectBehavioralAnomalies)
		r.Get("/behavior/{agentID}/similar", GetSimilarAgents)
		r.Post("/tenants", CreateTenant)
		r.Get("/tenants", ListTenants)
		r.Get("/tenants/{tenantID}", GetTenant)
		r.Get("/budget/check", CheckBudget)
		r.Post("/budget/spend", RecordSpend)
		r.Post("/push/register", RegisterPushToken)
		r.Get("/push/tokens", GetPushTokens)
		r.Delete("/push/tokens/{tokenID}", DeactivatePushToken)
	})

	return r
}

func TestHealthEndpoint(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMCPInitialize(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"id":      1,
	})

	resp, err := http.Post(ts.URL+"/v1/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
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

func TestMCPToolsList(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      2,
	})

	resp, err := http.Post(ts.URL+"/v1/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("MCP tools/list failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	resultData, _ := result["result"].(map[string]interface{})
	_, hasTools := resultData["tools"]
	if !hasTools {
		t.Error("expected tools in tools/list response")
	}
}

func TestMCPResourcesList(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "resources/list",
		"id":      3,
	})

	resp, err := http.Post(ts.URL+"/v1/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("MCP resources/list failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	resultData, _ := result["result"].(map[string]interface{})
	_, hasResources := resultData["resources"]
	if !hasResources {
		t.Error("expected resources in resources/list response")
	}
}

func TestMCPMethodNotFound(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "nonexistent/method",
		"id":      3,
	})

	resp, err := http.Post(ts.URL+"/v1/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	rpcErr, _ := result["error"].(map[string]interface{})
	if rpcErr == nil {
		t.Fatal("expected error in response for unknown method")
	}
	code, _ := rpcErr["code"].(float64)
	if int(code) != -32601 {
		t.Errorf("expected error code -32601, got %d", int(code))
	}
}

func TestMCPInvalidJSON(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/mcp", "application/json", bytes.NewReader([]byte("invalid json")))
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	rpcErr, _ := result["error"].(map[string]interface{})
	if rpcErr == nil {
		t.Fatal("expected error in response for invalid JSON")
	}
}

func TestRegisterAgentValidation(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{"missing name", map[string]interface{}{"owner": "team"}, http.StatusBadRequest},
		{"missing owner", map[string]interface{}{"name": "agent1"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			resp, err := http.Post(ts.URL+"/v1/agents/register", "application/json", bytes.NewReader(b))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestRegisterAgentSuccess(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			if strings.Contains(sql, "COUNT(*)") {
				return &mockRow{vals: []interface{}{0}}
			}
			if strings.Contains(sql, "api_keys") {
				return &mockRow{vals: []interface{}{time.Now()}}
			}
			return &mockRow{
				vals: []interface{}{
					time.Now(),
				},
			}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":          "test-agent",
		"owner":         "test-team",
		"public_key":    "Px0i2rDYwBKKDYICrOgaLRb+AqOoydHQalPjYzZe3i4=",
		"capabilities":  []string{"mcp"},
		"allowed_tools": []string{"read"},
	})
	resp, err := http.Post(ts.URL+"/v1/agents/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["agent_id"]; !ok {
		t.Error("expected agent_id in response")
	}
}

func TestRegisterAgentDBError(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":       "test-agent",
		"owner":      "test-team",
		"public_key": "Px0i2rDYwBKKDYICrOgaLRb+AqOoydHQalPjYzZe3i4=",
	})
	resp, err := http.Post(ts.URL+"/v1/agents/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 for DB error, got %d", resp.StatusCode)
	}
}

func TestAuthorizeMissingFields(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"resource_id": "res-1",
	})

	resp, err := http.Post(ts.URL+"/v1/authorize", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing agent_id/action, got %d", resp.StatusCode)
	}
}

func TestAuthorizeAgentNotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("not found")}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id":    "nonexistent",
		"action":      "read",
		"resource_id": "res-1",
	})
	resp, err := http.Post(ts.URL+"/v1/authorize", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if allowed, _ := result["allowed"].(bool); allowed {
		t.Error("expected allowed=false for nonexistent agent")
	}
}

func TestVerifySignatureMissingFields(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("not found")}
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id":  "agent-1",
		"message":   "hello",
		"signature": "c2ln",
	})

	resp, err := http.Post(ts.URL+"/v1/verify-signature", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent agent, got %d", resp.StatusCode)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: errors.New("not found")}
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/agents/nonexistent-id")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRegisterResourceValidation(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"type": "api",
	})

	resp, err := http.Post(ts.URL+"/v1/resources/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name/endpoint, got %d", resp.StatusCode)
	}
}

func TestDelegateAgentValidation(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"parent_agent_id": "agent-1",
	})

	resp, err := http.Post(ts.URL+"/v1/delegate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing child_agent_id, got %d", resp.StatusCode)
	}
}

func TestHITLRequestValidation(t *testing.T) {
	r := setupTestRouter(nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": "agent-1",
	})

	resp, err := http.Post(ts.URL+"/v1/hitl/request", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing action, got %d", resp.StatusCode)
	}
}

func TestListAgentsEmpty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: nil}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/agents")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	agents, exists := result["agents"]
	if !exists {
		t.Error("expected agents key in response")
	}
	_ = agents
}

func TestListResourcesEmpty(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: nil}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/resources")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
