package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

func setupAirportRouter(q *mockQuerier) http.Handler {
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

	r.Route("/v1", func(r chi.Router) {
		r.Post("/airport/heartbeat", AirportHeartbeatHandler)
		r.Get("/airport/agents", AirportSearchHandler)
		r.Get("/airport/online", AirportListOnlineHandler)
		r.Get("/airport/agents/{agentID}", AirportGetProfileHandler)
		r.Put("/airport/agents/{agentID}", AirportUpdateProfileHandler)
		r.Get("/airport/connections", AirportConnectionsHandler)
	})

	return r
}

func TestAirportHeartbeat(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	agentID := uuid.New().String()
	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": agentID,
		"status":   "online",
	})

	resp, err := http.Post(ts.URL+"/v1/airport/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
}

func TestAirportHeartbeatInvalidAgentID(t *testing.T) {
	q := &mockQuerier{}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": "not-a-uuid",
		"status":   "online",
	})

	resp, err := http.Post(ts.URL+"/v1/airport/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAirportHeartbeatDefaultStatus(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	agentID := uuid.New().String()
	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": agentID,
		"status":   "unknown-status",
	})

	resp, err := http.Post(ts.URL+"/v1/airport/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "online" {
		t.Errorf("expected status=online (default), got %v", result["status"])
	}
}

func TestAirportSearch(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/airport/agents")
	if err != nil {
		t.Fatalf("search request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	agents, ok := result["agents"].([]interface{})
	if !ok {
		t.Errorf("expected agents array, got %v", result["agents"])
	}
	if len(agents) != 0 {
		t.Errorf("expected empty agents, got %d", len(agents))
	}
}

func TestAirportListOnline(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/airport/online")
	if err != nil {
		t.Fatalf("online list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAirportConnections(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	agentID := uuid.New().String()
	resp, err := http.Get(ts.URL + "/v1/airport/connections?agent_id=" + agentID)
	if err != nil {
		t.Fatalf("connections request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAirportConnectionsMissingAgentID(t *testing.T) {
	q := &mockQuerier{}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/airport/connections")
	if err != nil {
		t.Fatalf("connections request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAirportUpdateProfile(t *testing.T) {
	q := &mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	agentID := uuid.New().String()
	body, _ := json.Marshal(map[string]interface{}{
		"description": "Test agent at the airport",
		"tags":        []string{"test", "demo"},
		"listed":      true,
	})

	req, _ := http.NewRequest("PUT", ts.URL+"/v1/airport/agents/"+agentID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("update profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["ok"] != true {
		t.Errorf("expected ok=true, got %v", result["ok"])
	}
}

func TestAirportUpdateProfileInvalidAgentID(t *testing.T) {
	q := &mockQuerier{}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"description": "Test",
	})

	req, _ := http.NewRequest("PUT", ts.URL+"/v1/airport/agents/not-a-uuid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("update profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAirportGetProfileNotFound(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &mockRow{scanErr: context.DeadlineExceeded}
		},
	}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	agentID := uuid.New().String()
	resp, err := http.Get(ts.URL + "/v1/airport/agents/" + agentID)
	if err != nil {
		t.Fatalf("get profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAirportGetProfileInvalidAgentID(t *testing.T) {
	q := &mockQuerier{}
	r := setupAirportRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/airport/agents/not-a-uuid")
	if err != nil {
		t.Fatalf("get profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}