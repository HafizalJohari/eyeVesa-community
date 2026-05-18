package tx

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func generateTestKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, priv
}

func TestIssueToken_Success(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy", "read"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if token.Subject != "agent-1" {
		t.Fatalf("expected subject 'agent-1', got %q", token.Subject)
	}
	if token.ResourceID != "res-1" {
		t.Fatalf("expected resource_id 'res-1', got %q", token.ResourceID)
	}
	if token.Action != "deploy" {
		t.Fatalf("expected action 'deploy', got %q", token.Action)
	}
	if token.TrustScore != 0.85 {
		t.Fatalf("expected trust 0.85, got %f", token.TrustScore)
	}
	if len(token.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(token.Scopes))
	}
	if token.Signature == "" {
		t.Fatal("expected non-empty signature")
	}
	if token.ExpiresAt <= token.IssuedAt {
		t.Fatal("expected expires_at > issued_at")
	}
	if token.ID == "" {
		t.Fatal("expected non-empty token ID")
	}
	if token.Nonce == "" {
		t.Fatal("expected non-empty nonce")
	}
}

func TestIssueToken_WithSkills(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	skills := []SkillClaim{
		{SkillID: "skill-1", SkillName: "kubernetes", Proficiency: 3, Verified: true},
	}
	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.9, []string{"deploy"}, skills, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if len(token.AgentSkills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(token.AgentSkills))
	}
	if token.AgentSkills[0].SkillName != "kubernetes" {
		t.Fatalf("expected skill 'kubernetes', got %q", token.AgentSkills[0].SkillName)
	}
	if !token.AgentSkills[0].Verified {
		t.Fatal("expected verified=true")
	}
}

func TestVerifyToken_Valid(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if err := svc.VerifyToken(token); err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, -1*time.Second)

	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := svc.VerifyToken(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	pub1, priv1 := generateTestKeys(t)
	_, priv2, _ := ed25519.GenerateKey(nil)

	svc1 := NewTokenService(priv1, pub1, 5*time.Minute)
	svc2 := NewTokenService(priv2, nil, 5*time.Minute)

	token, err := svc1.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	svc2.publicKey = pub1

	if err := svc2.VerifyToken(token); err != nil {
		t.Fatalf("VerifyToken with same key: %v", err)
	}

	differentpub, _, _ := ed25519.GenerateKey(nil)
	svc2.publicKey = differentpub

	if err := svc2.VerifyToken(token); err == nil {
		t.Fatal("expected error for wrong public key")
	}
}

func TestVerifyToken_NilToken(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	if err := svc.VerifyToken(nil); err == nil {
		t.Fatal("expected error for nil token")
	}
}

func TestVerifyToken_NoSignature(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token := &CapabilityToken{
		ID:         "test",
		Issuer:     "agentid-gateway",
		Subject:    "agent-1",
		ResourceID: "res-1",
		Action:     "deploy",
		IssuedAt:   time.Now().Unix(),
		ExpiresAt:  time.Now().Add(5 * time.Minute).Unix(),
	}

	if err := svc.VerifyToken(token); err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestDecodeAndVerifyToken(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	decoded, err := svc.DecodeAndVerifyToken(string(tokenJSON))
	if err != nil {
		t.Fatalf("DecodeAndVerifyToken: %v", err)
	}
	if decoded.Subject != "agent-1" {
		t.Fatalf("expected subject 'agent-1', got %q", decoded.Subject)
	}
}

func TestDecodeAndVerifyToken_InvalidJSON(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	_, err := svc.DecodeAndVerifyToken("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIssueReceipt(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, _ := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)

	receipt, err := svc.IssueReceipt(token, true, 0.86, 0.01)
	if err != nil {
		t.Fatalf("IssueReceipt: %v", err)
	}

	if receipt.TokenID != token.ID {
		t.Fatalf("expected token_id %q, got %q", token.ID, receipt.TokenID)
	}
	if receipt.AgentID != "agent-1" {
		t.Fatalf("expected agent_id 'agent-1', got %q", receipt.AgentID)
	}
	if !receipt.Allowed {
		t.Fatal("expected allowed=true")
	}
	if receipt.Signature == "" {
		t.Fatal("expected non-empty receipt signature")
	}
	if receipt.ReceiptID == "" {
		t.Fatal("expected non-empty receipt ID")
	}
}

func TestVerifyReceipt(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, _ := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	receipt, _ := svc.IssueReceipt(token, true, 0.86, 0.01)

	if err := svc.VerifyReceipt(receipt); err != nil {
		t.Fatalf("VerifyReceipt: %v", err)
	}
}

func TestVerifyReceipt_WrongKey(t *testing.T) {
	pub1, priv1 := generateTestKeys(t)
	svc := NewTokenService(priv1, pub1, 5*time.Minute)

	token, _ := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	receipt, _ := svc.IssueReceipt(token, true, 0.86, 0.01)

	differentpub, _, _ := ed25519.GenerateKey(nil)
	svc2 := NewTokenService(nil, differentpub, 5*time.Minute)

	if err := svc2.VerifyReceipt(receipt); err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestVerifyReceipt_Nil(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	if err := svc.VerifyReceipt(nil); err == nil {
		t.Fatal("expected error for nil receipt")
	}
}

func TestTokenExpiry(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 10*time.Minute)

	if svc.TokenExpiry() != 10*time.Minute {
		t.Fatalf("expected 10m expiry, got %v", svc.TokenExpiry())
	}
}

func TestDefaultTokenExpiry(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 0)

	if svc.TokenExpiry() != 5*time.Minute {
		t.Fatalf("expected default 5m expiry, got %v", svc.TokenExpiry())
	}
}

func TestTamperedToken(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	token, err := svc.IssueToken("agent-1", "res-1", "deploy", 0.85, []string{"deploy"}, nil, nil)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	token.TrustScore = 0.99

	if err := svc.VerifyToken(token); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestRoundTripTokenSerialization(t *testing.T) {
	pub, priv := generateTestKeys(t)
	svc := NewTokenService(priv, pub, 5*time.Minute)

	skills := []SkillClaim{
		{SkillID: "skill-1", SkillName: "kubernetes", Proficiency: 4, Verified: true},
		{SkillID: "skill-2", SkillName: "database", Proficiency: 2, Verified: false},
	}
	params := map[string]interface{}{"namespace": "production"}

	token, err := svc.IssueToken("agent-1", "res-1", "k8s_deploy", 0.9, []string{"deploy", "read"}, skills, params)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded CapabilityToken
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if err := svc.VerifyToken(&decoded); err != nil {
		t.Fatalf("VerifyToken after round-trip: %v", err)
	}

	if decoded.Subject != "agent-1" {
		t.Fatalf("expected 'agent-1', got %q", decoded.Subject)
	}
	if len(decoded.AgentSkills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(decoded.AgentSkills))
	}
}