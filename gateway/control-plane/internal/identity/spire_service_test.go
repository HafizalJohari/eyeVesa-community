package identity

import (
	"encoding/json"
	"testing"
)

func TestValidateBundle_JWKSet(t *testing.T) {
	jwkSet := `{"keys":[{"kty":"EC","crv":"P-256","x":"test","y":"test"}]}`
	if err := validateBundle(jwkSet); err != nil {
		t.Errorf("valid JWKSet should pass: %v", err)
	}
}

func TestValidateBundle_JWKSetMissingKeys(t *testing.T) {
	badJSON := `{"alg":"ES256"}`
	if err := validateBundle(badJSON); err == nil {
		t.Error("JWKSet without 'keys' should fail")
	}
}

func TestValidateBundle_InvalidJSON(t *testing.T) {
	if err := validateBundle("{invalid json"); err == nil {
		t.Error("invalid JSON should fail")
	}
}

func TestValidateBundle_PEM(t *testing.T) {
	pemData := `-----BEGIN CERTIFICATE-----
MIIBjTCCATOgAwIBAgIJAKLFHA7KQW3oMA0GCSqGSIb3DQEBCwUAMBUxEzARBgNV
BAMMCnNwaWZmZS5kZXYwHhcNMjQwMTAxMDAwMDAwWhcNMjUwMTAxMDAwMDAwWjAV
MRMwEQYDVQQDDApzcGlmZmUuZGV2MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
testP2Vktesttesttesttesttesttesttesttesttesttesttesttesttesttest
o2owa6N/MD4wDQYJKoZIhvcNAQELBQADQQBgJhI0F1M9testtesttesttesttest
-----END CERTIFICATE-----`
	if err := validateBundle(pemData); err == nil {
		t.Error("invalid PEM certificate data should fail")
	}
}

func TestValidateBundle_Empty(t *testing.T) {
	if err := validateBundle(""); err == nil {
		t.Error("empty bundle should fail")
	}
}

func TestValidateBundle_RandomString(t *testing.T) {
	if err := validateBundle("not a valid bundle format at all"); err == nil {
		t.Error("random string should fail")
	}
}

func TestParseSelectors_String(t *testing.T) {
	result, ok := parseSelectors(`["unix:uid:1000","k8s:ns:default"]`)
	if !ok {
		t.Fatal("should parse JSON array string")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 selectors, got %d", len(result))
	}
}

func TestParseSelectors_EmptyString(t *testing.T) {
	result, ok := parseSelectors("")
	if !ok {
		t.Fatal("should parse empty string")
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseSelectors_EmptyArray(t *testing.T) {
	result, ok := parseSelectors("[]")
	if !ok {
		t.Fatal("should parse empty array")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result))
	}
}

func TestParseSelectors_StringSlice(t *testing.T) {
	input := []string{"unix:uid:1000", "k8s:ns:default"}
	result, ok := parseSelectors(input)
	if !ok {
		t.Fatal("should parse string slice")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 selectors, got %d", len(result))
	}
}

func TestParseSelectors_InterfaceSlice(t *testing.T) {
	input := []interface{}{"unix:uid:1000", "k8s:ns:default"}
	result, ok := parseSelectors(input)
	if !ok {
		t.Fatal("should parse interface slice")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 selectors, got %d", len(result))
	}
}

func TestParseSelectors_InvalidType(t *testing.T) {
	_, ok := parseSelectors(42)
	if ok {
		t.Error("int should not parse as selectors")
	}
}

func TestSpireStatusDefaults(t *testing.T) {
	s := &SpireStatus{}
	if s.Available {
		t.Error("default status should not be available")
	}
	if s.BundleCount != 0 {
		t.Error("default bundle count should be 0")
	}
}

func TestTrustBundleJSON(t *testing.T) {
	b := TrustBundle{
		BundleID:       "test-id",
		TrustDomain:    "example.com",
		BundleType:     "spiffe_x509",
		Source:         "static",
		SequenceNumber: 1,
		IsFederated:    false,
		Verified:       false,
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("failed to marshal trust bundle: %v", err)
	}

	var decoded TrustBundle
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal trust bundle: %v", err)
	}

	if decoded.TrustDomain != "example.com" {
		t.Errorf("expected trust_domain=example.com, got %s", decoded.TrustDomain)
	}
	if decoded.BundleID != "test-id" {
		t.Errorf("expected bundle_id=test-id, got %s", decoded.BundleID)
	}
}

func TestWorkloadRegistrationJSON(t *testing.T) {
	w := WorkloadRegistration{
		RegistrationID: "reg-1",
		SpiffeID:      "spiffe://example.com/workload",
		AgentID:       "agent-1",
		TrustDomain:   "example.com",
		Selectors:     []string{"unix:uid:1000"},
		AutoRegister:  true,
		Status:        "active",
	}

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("failed to marshal workload: %v", err)
	}

	var decoded WorkloadRegistration
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal workload: %v", err)
	}

	if decoded.SpiffeID != "spiffe://example.com/workload" {
		t.Errorf("expected spiffe_id=spiffe://example.com/workload, got %s", decoded.SpiffeID)
	}
	if decoded.Status != "active" {
		t.Errorf("expected status=active, got %s", decoded.Status)
	}
}