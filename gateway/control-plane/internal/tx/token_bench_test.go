package tx

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func generateBenchKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return pub, priv
}

func benchIssueToken(svc *TokenService) *CapabilityToken {
	t, _ := svc.IssueToken("bench-agent", "bench-resource", "read", 0.85, []string{"read"}, nil, nil)
	return t
}

func BenchmarkTokenIssue(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchIssueToken(svc)
	}
}

func BenchmarkTokenVerify(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)
	token := benchIssueToken(svc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.VerifyToken(token)
	}
}

func BenchmarkTokenIssueAndVerify(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := benchIssueToken(svc)
		_ = svc.VerifyToken(token)
	}
}

func BenchmarkReceiptIssue(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)
	token := benchIssueToken(svc)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.IssueReceipt(token, true, 0.85, 0.02)
	}
}

func BenchmarkReceiptVerify(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)
	token := benchIssueToken(svc)
	receipt, _ := svc.IssueReceipt(token, true, 0.85, 0.02)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.VerifyReceipt(receipt)
	}
}

func BenchmarkDecodeAndVerifyToken(b *testing.B) {
	pubKey, privKey := generateBenchKeys()
	svc := NewTokenService(privKey, pubKey, 5*time.Minute)
	token := benchIssueToken(svc)
	tokenJSON, _ := json.Marshal(token)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.DecodeAndVerifyToken(string(tokenJSON))
	}
}