package crypto

import (
	"crypto/ed25519"
	"testing"
	"time"
)

func BenchmarkGenerateKeypair(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ed25519.GenerateKey(nil)
	}
}

func BenchmarkSign(b *testing.B) {
	_, privKey, _ := ed25519.GenerateKey(nil)
	msg := []byte("benchmark signing message for load test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ed25519.Sign(privKey, msg)
	}
}

func BenchmarkVerify(b *testing.B) {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	msg := []byte("benchmark signing message for load test")
	sig := ed25519.Sign(privKey, msg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ed25519.Verify(pubKey, msg, sig)
	}
}

func BenchmarkSignVerify(b *testing.B) {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	msg := []byte("benchmark signing message for load test")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sig := ed25519.Sign(privKey, msg)
		_ = ed25519.Verify(pubKey, msg, sig)
	}
}

func BenchmarkEncodeBase64(b *testing.B) {
	data := make([]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EncodeBase64(data)
	}
}

func BenchmarkLoadOrGenerateKeys(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyPath := b.TempDir() + "/bench.key"
		_, _, _ = LoadOrGenerateKeys(keyPath)
	}
}

func BenchmarkKeyRotationServiceSign(b *testing.B) {
	dir := b.TempDir()
	keyPath := dir + "/bench.key"
	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	msg := []byte("benchmark rotation sign")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.Sign(msg)
	}
}

func BenchmarkKeyRotationServiceVerify(b *testing.B) {
	dir := b.TempDir()
	keyPath := dir + "/bench.key"
	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	msg := []byte("benchmark rotation verify")
	sig := svc.Sign(msg)
	pubKey := svc.GetCurrentPublicKey()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.Verify(pubKey, msg, sig)
	}
}

func BenchmarkKeyRotationRotate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dir := b.TempDir()
		keyPath := dir + "/bench.key"
		svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
		_, _ = svc.Rotate()
	}
}