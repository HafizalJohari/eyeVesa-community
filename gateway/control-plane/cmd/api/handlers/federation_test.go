package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			if strings.Contains(sql, "federation_peer_invites") {
				return &flexMockRow{
					vals: []interface{}{
						uuid.New().String(),
						"test-gateway",
						"http://localhost:9443",
						"org:test",
						"community",
						time.Now().Add(24 * time.Hour),
					},
				}
			}
			return &flexMockRow{
				vals: []interface{}{
					uuid.New().String(),
					time.Now(),
				},
			}
		},
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
	SetSpireService(nil)
	SetIdentityProvider(nil)

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/federation/invites", CreateFederationInvite)
		r.Post("/federation/register", RegisterFederationPeer)
		r.Post("/federation/register-admin", RegisterFederationPeerAdmin)
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
		"gateway_id":       gatewayID,
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
		"name":         "test-gateway",
		"public_key":   pubKeyB64,
		"endpoint":     "http://localhost:9443",
		"trust_domain": "org:test",
		"invite_token": "test-invite-token",
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

func TestCreateFederationInvite(t *testing.T) {
	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &flexMockRow{
				vals: []interface{}{
					uuid.New().String(),
					time.Now().Add(24 * time.Hour),
					time.Now(),
				},
			}
		},
	}
	SetQuerier(q)

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Post("/v1/federation/invites", CreateFederationInvite)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":     "node-b",
		"endpoint": "http://localhost:9444",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/invites", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create invite request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("expected one-time invite token in response")
	}
}

func TestRegisterFederationPeerRequiresInviteOrAdminApproval(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	pubKey, _ := generateTestGatewayKeypair(t)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	body, _ := json.Marshal(map[string]interface{}{
		"name":       "test-gateway",
		"public_key": pubKeyB64,
		"endpoint":   "http://localhost:9443",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing invite/admin approval, got %d", resp.StatusCode)
	}
}

func TestRegisterFederationPeerRejectsInviteEndpointMismatch(t *testing.T) {
	r := setupFederationRouter()
	ts := httptest.NewServer(r)
	defer ts.Close()

	pubKey, _ := generateTestGatewayKeypair(t)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	body, _ := json.Marshal(map[string]interface{}{
		"name":         "test-gateway",
		"public_key":   pubKeyB64,
		"endpoint":     "http://localhost:9444",
		"trust_domain": "org:test",
		"invite_token": "test-invite-token",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invite endpoint mismatch, got %d", resp.StatusCode)
	}
}

func TestRegisterFederationPeerRejectsExpiredInvite(t *testing.T) {
	pubKey, _ := generateTestGatewayKeypair(t)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	q := &mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			if strings.Contains(sql, "federation_peer_invites") {
				return &flexMockRow{
					vals: []interface{}{
						uuid.New().String(),
						"test-gateway",
						"http://localhost:9443",
						"org:test",
						"community",
						time.Now().Add(-time.Hour),
					},
				}
			}
			return &flexMockRow{vals: []interface{}{uuid.New().String(), time.Now()}}
		},
	}
	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Post("/v1/federation/register", RegisterFederationPeer)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":         "test-gateway",
		"public_key":   pubKeyB64,
		"endpoint":     "http://localhost:9443",
		"trust_domain": "org:test",
		"invite_token": "expired-token",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for expired invite, got %d", resp.StatusCode)
	}
}

func TestRegisterFederationPeerRejectsReusedInvite(t *testing.T) {
	pubKey, _ := generateTestGatewayKeypair(t)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)

	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			if strings.Contains(sql, "federation_peer_invites") {
				return &mockRow{scanErr: errors.New("no rows")}
			}
			return &flexMockRow{vals: []interface{}{uuid.New().String(), time.Now()}}
		},
	})
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Post("/v1/federation/register", RegisterFederationPeer)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"name":         "test-gateway",
		"public_key":   pubKeyB64,
		"endpoint":     "http://localhost:9443",
		"trust_domain": "org:test",
		"invite_token": "already-used-token",
	})

	resp, err := http.Post(ts.URL+"/v1/federation/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register peer request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for reused invite, got %d", resp.StatusCode)
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
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			gwPubKey, _ := generateTestGatewayKeypair(t)
			return &flexMockRow{
				vals: []interface{}{
					uuid.New().String(),
					"test-gateway",
					[]byte(gwPubKey),
					"http://localhost:9443",
					"org:test",
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
				},
			}
		},
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

func TestFederatedHeartbeatRejectsSuspendedGateway(t *testing.T) {
	gwPubKey, _ := generateTestGatewayKeypair(t)
	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(&mockQuerier{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) database.Row {
			return &flexMockRow{
				vals: []interface{}{
					uuid.New().String(),
					"test-gateway",
					[]byte(gwPubKey),
					"http://localhost:9443",
					"org:test",
					"community",
					"suspended",
					0.2,
					0,
					nil,
					time.Now(),
				},
			}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			t.Fatal("suspended gateway heartbeat must not write")
			return database.CommandTag{}, nil
		},
	})
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Post("/v1/federation/heartbeat", FederatedHeartbeatHandler)
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

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for suspended gateway heartbeat, got %d", resp.StatusCode)
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

func TestSyncFederatedAgentRejectsStalePassport(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)
	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	sig := signPassport(t, gwPrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

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
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
				},
			}
		},
		execFn: func(ctx context.Context, sql string, args ...interface{}) (database.CommandTag, error) {
			t.Fatal("stale passport must not write federated agent")
			return database.CommandTag{}, nil
		},
	})
	SetFederationService(fs)

	r := chi.NewRouter()
	r.Post("/v1/federation/agents/sync", SyncFederatedAgent)
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"passport": map[string]interface{}{
			"agent_id":          agentID,
			"agent_public_key":  agentPubKeyB64,
			"gateway_id":        gatewayID,
			"gateway_signature": sig,
			"issued_at":         issuedAt,
		},
		"name":        "stale-agent",
		"owner":       "org:test",
		"trust_score": 0.9,
	})

	resp, err := http.Post(ts.URL+"/v1/federation/agents/sync", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("sync request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for stale passport, got %d", resp.StatusCode)
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

func TestSearchFederatedAgentsFiltersSuspendedPeers(t *testing.T) {
	var capturedSQL string
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			capturedSQL = sql
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)

	if _, err := fs.SearchFederatedAgents(context.Background(), "", "", "", 0, "", 50, 0); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if !strings.Contains(capturedSQL, "fg.status = 'active'") {
		t.Fatalf("expected federated search to filter active peers, query was: %s", capturedSQL)
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

func TestListFederatedOnlineFiltersSuspendedPeers(t *testing.T) {
	var capturedSQL string
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			capturedSQL = sql
			return &mockRows{results: [][]interface{}{}}, nil
		},
	}
	fs := identity.NewFederationService(nil)
	fs.SetQuerierForTest(q)

	if _, err := fs.ListFederatedOnline(context.Background(), ""); err != nil {
		t.Fatalf("online list failed: %v", err)
	}
	if !strings.Contains(capturedSQL, "fg.status = 'active'") {
		t.Fatalf("expected federated online list to filter active peers, query was: %s", capturedSQL)
	}
}

func TestPassportVerification(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)

	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := time.Now().Format(time.RFC3339)

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
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
				},
			}
		},
	})

	verifyErr := fs.VerifyPassport(context.Background(), passport)
	if verifyErr != nil {
		t.Errorf("passport verification should succeed: %v", verifyErr)
	}
}

func TestPassportVerificationRejectsStaleIssuedAt(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)
	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	sig := signPassport(t, gwPrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

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
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
				},
			}
		},
	})

	err := fs.VerifyPassport(context.Background(), identity.AgentPassport{
		AgentID:     agentID,
		AgentPubKey: agentPubKeyB64,
		GatewayID:   gatewayID,
		GatewaySig:  sig,
		IssuedAt:    issuedAt,
	})
	if err == nil {
		t.Fatal("passport verification should fail for stale issued_at")
	}
}

func TestPassportVerificationRejectsFutureIssuedAt(t *testing.T) {
	gwPubKey, gwPrivKey := generateTestGatewayKeypair(t)
	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	sig := signPassport(t, gwPrivKey, agentID, agentPubKeyB64, gatewayID, issuedAt)

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
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
				},
			}
		},
	})

	err := fs.VerifyPassport(context.Background(), identity.AgentPassport{
		AgentID:     agentID,
		AgentPubKey: agentPubKeyB64,
		GatewayID:   gatewayID,
		GatewaySig:  sig,
		IssuedAt:    issuedAt,
	})
	if err == nil {
		t.Fatal("passport verification should fail for future issued_at")
	}
}

func TestPassportVerificationInvalidSignature(t *testing.T) {
	gwPubKey, _ := generateTestGatewayKeypair(t)
	_, fakePrivKey := generateTestGatewayKeypair(t)

	agentPubKey, _, _ := ed25519.GenerateKey(nil)
	agentPubKeyB64 := base64.StdEncoding.EncodeToString(agentPubKey)
	agentID := uuid.New().String()
	gatewayID := uuid.New().String()
	issuedAt := time.Now().Format(time.RFC3339)

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
					"community",
					"active",
					1.0,
					0,
					nil,
					time.Now(),
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
	issuedAt := time.Now().Format(time.RFC3339)

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
					"community",
					"suspended",
					0.5,
					0,
					nil,
					time.Now(),
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
