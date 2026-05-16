package ptv

import (
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
	if len(proof.TPMSignature) == 0 {
		t.Fatal("TPMSignature is empty")
	}
	if len(proof.Quote) == 0 {
		t.Fatal("Quote is empty")
	}
}