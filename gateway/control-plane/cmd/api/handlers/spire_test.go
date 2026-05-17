package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
)

type mockIdentityProvider struct {
	svid *identity.SVID
	err  error
}

func (m *mockIdentityProvider) FetchSVID(ctx context.Context) (*identity.SVID, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.svid, nil
}

func (m *mockIdentityProvider) WriteCerts(certPath, keyPath string) error {
	return nil
}

func setupSpireRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Post("/v1/spire/bundles", CreateTrustBundle)
	r.Get("/v1/spire/bundles", ListTrustBundles)
	r.Get("/v1/spire/bundles/{trustDomain}", GetTrustBundle)
	r.Put("/v1/spire/bundles/{trustDomain}", UpdateTrustBundle)
	r.Post("/v1/spire/bundles/{trustDomain}/verify", VerifyTrustBundle)
	r.Delete("/v1/spire/bundles/{trustDomain}", DeleteTrustBundle)
	r.Post("/v1/spire/bundles/fetch", FetchBundleFromEndpoint)
	r.Post("/v1/spire/workloads", RegisterWorkload)
	r.Get("/v1/spire/workloads", ListWorkloads)
	r.Get("/v1/spire/workloads/{spiffeID}", GetWorkload)
	r.Post("/v1/spire/workloads/{spiffeID}/attest", AttestWorkload)
	r.Delete("/v1/spire/workloads/{spiffeID}", DeleteWorkload)
	r.Get("/v1/spire/status", GetSpireStatus)
	return r
}

func TestCreateTrustBundle_MissingFields(t *testing.T) {
	spireService = nil
	identityProvider = nil

	r := setupSpireRouter()

	body, _ := json.Marshal(map[string]string{"trust_domain": "example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/spire/bundles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	body2, _ := json.Marshal(map[string]string{"bundle_data": "testdata"})
	req2 := httptest.NewRequest(http.MethodPost, "/v1/spire/bundles", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing trust_domain, got %d", w2.Code)
	}
}

func TestRegisterWorkload_MissingFields(t *testing.T) {
	spireService = nil
	identityProvider = nil

	r := setupSpireRouter()

	body, _ := json.Marshal(map[string]string{"spiffe_id": "spiffe://example.com/workload"})
	req := httptest.NewRequest(http.MethodPost, "/v1/spire/workloads", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing agent_id and trust_domain, got %d", w.Code)
	}
}

func TestGetSpireStatus_NoProvider(t *testing.T) {
	spireService = nil
	identityProvider = nil

	r := setupSpireRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/spire/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["available"] != false {
		t.Error("expected available=false with no provider")
	}
}

func TestFetchBundleFromEndpoint_MissingURL(t *testing.T) {
	spireService = nil
	identityProvider = nil

	r := setupSpireRouter()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/v1/spire/bundles/fetch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing endpoint_url, got %d", w.Code)
	}
}

func TestCreateTrustBundle_InvalidBody(t *testing.T) {
	spireService = nil
	identityProvider = nil

	r := setupSpireRouter()

	req := httptest.NewRequest(http.MethodPost, "/v1/spire/bundles", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}