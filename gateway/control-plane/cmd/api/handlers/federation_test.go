package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

func setupFederationRouter() http.Handler {
	q := &mockQuerier{}
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
	SetSpireService(nil)
	SetIdentityProvider(nil)

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/federation/register", RegisterFederationPeer)
		r.Get("/federation/peers", ListFederationPeers)
		r.Get("/federation/peers/{gatewayID}", GetFederationPeer)
		r.Post("/federation/agents/sync", SyncFederatedAgent)
		r.Post("/federation/heartbeat", FederatedHeartbeatHandler)
		r.Get("/federation/agents", SearchFederatedAgentsHandler)
		r.Get("/federation/online", ListFederatedOnlineHandler)
		r.Get("/federation/agents/{agentID}", GetFederatedAgentHandler)
		r.Post("/federation/peers/{gatewayID}/suspend", SuspendFederationPeerHandler)
		r.Get("/federation/health", FederationHealthHandler)
	})

	return r
}

func generateTestGatewayKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	return pubKey, privKey
}

func signPassport(t *testing.T, privKey ed25519.PrivateKey, agentID, agentPubKeyB64, gatewayID, issuedAt string) string {
	t.Helper()
	payload := map[string]string{
		"agent_id":         agentID,
		"agent_public_key": agentPubKeyB64,
		"gateway_id":      gatewayID,
		"issued_at":        issuedAt,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal passport payload: %v", err)
	}
	sig := ed25519.Sign(privKey, payloadBytes)
	return base64.StdEncoding.EncodeToString(sig)
}

func TestRegisterFederationPeer(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	pubKey, _ := generateTestGatewayKeypair(t)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "test-gateway",
		"public_key":  pubKeyB64,
		"endpoint":    "http://localhost:9443",
		"trust_domain": "org:test",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["name"] != "test-gateway" {
		t.Errorf("expected name=test-gateway, got %v", result["name"])
	}
	if result["status"] != "active" {
		t.Errorf("expected status=active, got %v", result["status"])
	}
	if _, ok := result["gateway_id"]; !ok {
		t.Error("expected gateway_id in response")
	}
}

func TestRegisterFederationPeerMissingFields(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name": "test-gateway",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestRegisterFederationPeerInvalidPublicKey(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":       "test-gateway",
		"public_key": "not-a-valid-key",
		"endpoint":   "http://localhost:9443",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid public_key, got %d", resp.StatusCode)
	}
}

func TestListFederationPeers(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	SetQuerier(q)

	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/federation/peers")
	if err != nil {
		t.Fatalf("list peers request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["peers"]; !ok {
		t.Error("expected peers in response")
	}
}

func TestFederationHealth(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/federation/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", result["status"])
	}
}

func TestFederatedHeartbeat(t *testing.T) {
	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			return database.CommandTag{RowsAffected: 1}, nil
		},
	})
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Route("/v1", func(r chi.Router) {
		r.Post("/federation/heartbeat", FederatedHeartbeatHandler)
	})
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id":   uuid.New().String(),
		"gateway_id": uuid.New().String(),
		"status":     "online",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/heartbeat", "application/json", bytes.NewReader(body))
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

func TestFederatedHeartbeatMissingFields(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": uuid.New().String(),
	})

	resp, err := http.Post(ts.URL+"/v1/federation/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing gateway_id, got %d", resp.StatusCode)
	}
}

func TestSyncFederatedAgentMissingPassport(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":  "test-agent",
		"owner": "org:test",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/agents/sync", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing passport, got %d", resp.StatusCode)
	}
}

func TestSearchFederatedAgents(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	SetQuerier(q)

	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/federation/agents")
	if err != nil {
		t.Fatalf("search request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["agents"]; !ok {
		t.Error("expected agents in response")
	}
}

func TestListFederatedOnline(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	SetQuerier(q)

	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/federation/online")
	if err != nil {
		t.Fatalf("online list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPassportVerification(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)

	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := "2026-01-01T00:00:00Z"

	sig := signPassport(t, gwPrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

	passport := identity.AgentPassport{
		AgentID:     agentID,
		AgentPubKey: agentPubKeyB64,
		GatewayID:   gatewayID,
		GatewaySig:  sig,
		IssuedAt:    issuedAt,
	}

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &flexMockRow{
				vals: []interface{}{
					gatewayID,
					"test-gateway",
					[]byte(gwPubKey),
					"http://localhost:9443",
					"org:test",
					"active",
					1.0,
					0,
					nil,
					"2026-01-01T00:00:00Z",
				},
			}
		},
	})

	verifyErr := fs.VerifyPassport(context.Background(), passport)
	if verifyErr != nil {
		t.Errorf("passport verification should succeed: %v", verifyErr)
	}
}

func TestPassportVerificationInvalidSignature(t *testing.T) {
	gwPubKey, _ := generateTestGatewayKeypair(t)
	_, fakePrivKey := generateTestGatewayKeypair(t)

	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := "2026-01-01T00:00:00Z"

	sig := signPassport(t, fakePrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

	passport := identity.AgentPassport{
		AgentID:     agentID,
		AgentPubKey: agentPubKeyB64,
		GatewayID:   gatewayID,
		GatewaySig:  sig,
		IssuedAt:    issuedAt,
	}

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &flexMockRow{
				vals: []interface{}{
					gatewayID,
					"test-gateway",
					[]byte(gwPubKey),
					"http://localhost:9443",
					"org:test",
					"active",
					1.0,
					0,
					nil,
					"2026-01-01T00:00:00Z",
				},
			}
		},
	})

	err := fs.VerifyPassport(context.Background(), passport)
	if err == nil {
		t.Error("passport verification should fail with wrong signature")
	}
}

func TestPassportVerificationSuspendedGateway(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)

	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := "2026-01-01T00:00:00Z"

	sig := signPassport(t, gwPrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

	passport := identity.AgentPassport{
		AgentID:     agentID,
		AgentPubKey: agentPubKeyB64,
		GatewayID:   gatewayID,
		GatewaySig:  sig,
		IssuedAt:    issuedAt,
	}

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &flexMockRow{
				vals: []interface{}{
					gatewayID,
					"test-gateway",
					[]byte(gwPubKey),
					"http://localhost:9443",
					"org:test",
					"suspended",
					0.5,
					0,
					nil,
					"2026-01-01T00:00:00Z",
				},
			}
		},
	})

	suspendErr := fs.VerifyPassport(context.Background(), passport)
	if suspendErr == nil {
		t.Error("passport verification should fail for suspended gateway")
	}
}

type flexMockRow struct {
	vals []interface{}
}

func (r *flexMockRow) Scan(dest ...interface{}) error {
	for i, v := range r.vals {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *string:
				if s, ok := v.(string); ok {
					*d = s
				}
			case *[]byte:
				if b, ok := v.([]byte); ok {
					*d = b
				}
			case *float64:
				if f, ok := v.(float64); ok {
					*d = f
				}
			case *int:
				if n, ok := v.(int); ok {
					*d = n
				}
			case *time.Time:
				if t, ok := v.(time.Time); ok {
					*d = t
				}
			case **time.Time:
				if t, ok := v.(*time.Time); ok {
					*d = t
				} else if v == nil {
					*d = nil
				}
			}
		}
	}
	return nil
}