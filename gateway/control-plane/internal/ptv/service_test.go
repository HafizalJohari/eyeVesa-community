package ptv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"
)

func TestNewPTVService(t *testing.T) {
	svc := NewPTVService(nil)
	if svc == nil {
		t.Fatal("NewPTVService returned nil")
	}
	if svc.gatewayPrivKey == nil {
		t.Fatal("PTVService private key is nil")
	}
}

func TestPTVServiceProve(t *testing.T) {
	svc := NewPTVService(nil)
	proof, err := svc.Prove(nil, "agent-1", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))
	if err != nil {
		t.Fatalf("Prove failed: %v", err)
	}
	if proof == nil {
		t.Fatal("Proof is nil")
	}
	if proof.Attestation.AgentID != "agent-1" {
		t.Fatalf("AgentID mismatch: got %s", proof.Attestation.AgentID)
	}
	if proof.Attestation.Platform != "macos" {
		t.Fatalf("Platform mismatch: got %s", proof.Attestation.Platform)
	}
	if proof.Attestation.FirmwareVersion != "1.0" {
		t.Fatalf("FirmwareVersion mismatch: got %s", proof.Attestation.FirmwareVersion)
	}
	if len(proof.TPMSignature) == 0 {
		t.Fatal("TPMSignature is empty")
	}
	if len(proof.Quote) == 0 {
		t.Fatal("Quote is empty")
	}
	if proof.Attestation.Timestamp == 0 {
		t.Fatal("Timestamp should not be zero")
	}
}

func TestPTVServiceProve_MultipleAgents(t *testing.T) {
	svc := NewPTVService(nil)
	proof1, _ := svc.Prove(nil, "agent-1", "linux", "2.0", []byte("key1"), []byte("hash1"), []byte("nonce1"))
	proof2, _ := svc.Prove(nil, "agent-2", "windows", "3.0", []byte("key2"), []byte("hash2"), []byte("nonce2"))

	if proof1.Attestation.AgentID == proof2.Attestation.AgentID {
		t.Fatal("different agent IDs should produce different attestation agents")
	}
	if string(proof1.TPMSignature) == string(proof2.TPMSignature) {
		t.Fatal("different inputs should produce different signatures")
	}
}

func TestPTVServiceProve_SignatureVerify(t *testing.T) {
	svc := NewPTVService(nil)
	proof, _ := svc.Prove(nil, "agent-1", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))

	attBytes, _ := json.Marshal(proof.Attestation)
	hash := sha256.Sum256(attBytes)
	if !ecdsa.VerifyASN1(&svc.gatewayPrivKey.PublicKey, hash[:], proof.TPMSignature) {
		t.Fatal("Prove signature should verify against gateway public key")
	}
}

func TestPTVServiceProve_TamperedSignature(t *testing.T) {
	svc := NewPTVService(nil)
	proof, _ := svc.Prove(nil, "agent-1", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))

	attBytes, _ := json.Marshal(proof.Attestation)
	hash := sha256.Sum256(attBytes)

	proof.TPMSignature[0] ^= 0xFF
	if ecdsa.VerifyASN1(&svc.gatewayPrivKey.PublicKey, hash[:], proof.TPMSignature) {
		t.Fatal("tampered signature should NOT verify")
	}
}

func TestPTVServiceProve_WrongKey(t *testing.T) {
	svc := NewPTVService(nil)
	proof, _ := svc.Prove(nil, "agent-1", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))

	otherKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	attBytes, _ := json.Marshal(proof.Attestation)
	hash := sha256.Sum256(attBytes)
	if ecdsa.VerifyASN1(&otherKey.PublicKey, hash[:], proof.TPMSignature) {
		t.Fatal("signature should NOT verify with wrong public key")
	}
}

func TestPTVService_QuoteDerivation(t *testing.T) {
	svc := NewPTVService(nil)
	proof, _ := svc.Prove(nil, "agent-1", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))

	attBytes, _ := json.Marshal(proof.Attestation)
	expectedQuote := sha256.Sum256(append(attBytes, proof.TPMSignature...))
	if string(proof.Quote) != string(expectedQuote[:]) {
		t.Fatal("Quote should be SHA256(attestation + signature)")
	}
}

func TestNewPTVService_KeyPersistence(t *testing.T) {
	keyPath := t.TempDir() + "/test-ptv-key.pem"
	os.Setenv("PTV_KEY_PATH", keyPath)
	defer os.Unsetenv("PTV_KEY_PATH")

	svc1 := NewPTVService(nil)
	svc2 := NewPTVService(nil)

	if svc1.gatewayPrivKey.D.Cmp(svc2.gatewayPrivKey.D) != 0 {
		t.Fatal("key should be persisted and reloaded from same path")
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("key file should exist: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("key file should contain valid PEM")
	}
}

func TestNewPTVService_InvalidKeyFile(t *testing.T) {
	keyPath := t.TempDir() + "/invalid-key.pem"
	os.WriteFile(keyPath, []byte("not-a-valid-key"), 0600)
	os.Setenv("PTV_KEY_PATH", keyPath)
	defer os.Unsetenv("PTV_KEY_PATH")

	svc := NewPTVService(nil)
	if svc == nil || svc.gatewayPrivKey == nil {
		t.Fatal("should generate new key when file is invalid")
	}
}

func TestVerificationResult_Fields(t *testing.T) {
	v := &VerificationResult{
		Valid:      true,
		AgentID:    "agent-1",
		Platform:   "macos",
		Message:    "valid",
		VerifiedAt: 1700000000,
	}
	if !v.Valid {
		t.Fatal("Valid should be true")
	}
	if v.AgentID != "agent-1" {
		t.Fatalf("AgentID mismatch: got %s", v.AgentID)
	}
}

func TestHardwareAttestation_Fields(t *testing.T) {
	a := HardwareAttestation{
		AgentID:         "a1",
		Platform:        "linux",
		FirmwareVersion: "1.0",
		TPMPublicKey:    []byte("key"),
		RuntimeHash:     []byte("hash"),
		Timestamp:       1700000000,
		Nonce:           []byte("nonce"),
	}
	if a.AgentID != "a1" {
		t.Fatalf("AgentID mismatch: got %s", a.AgentID)
	}
}

func TestIdentityBinding_Fields(t *testing.T) {
	b := IdentityBinding{
		BindingID:        "b1",
		AgentID:         "a1",
		AgentPublicKey:   []byte("apk"),
		HardwarePublicKey: []byte("hpk"),
		Platform:        "macos",
		RuntimeHash:      []byte("rh"),
		TransformedAt:   1700000000,
		BindingSignature: []byte("sig"),
		ExpiresAt:        1700003600,
	}
	if b.BindingID != "b1" {
		t.Fatalf("BindingID mismatch: got %s", b.BindingID)
	}
}

func TestPTVService_PublicKeyMatches(t *testing.T) {
	svc := NewPTVService(nil)
	pub := &svc.gatewayPrivKey.PublicKey
	if pub == nil {
		t.Fatal("public key should not be nil")
	}
	if pub.Curve != elliptic.P256() {
		t.Fatalf("expected P-256 curve, got %v", pub.Curve)
	}
}

func TestECDSAPublicKeySerialization(t *testing.T) {
	svc := NewPTVService(nil)
	pubKey := &svc.gatewayPrivKey.PublicKey

	pubDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	block, _ := pem.Decode(pubPEM)
	if block == nil {
		t.Fatal("failed to decode public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	parsedEC, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("parsed key is not ECDSA")
	}
	if parsedEC.X.Cmp(pubKey.X) != 0 || parsedEC.Y.Cmp(pubKey.Y) != 0 {
		t.Fatal("public key round-trip failed")
	}
}