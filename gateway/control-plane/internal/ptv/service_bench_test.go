package ptv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func BenchmarkPTVProve(b *testing.B) {
	svc := NewPTVService(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Prove(nil, "bench-agent", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))
	}
}

func BenchmarkPTVProveParallel(b *testing.B) {
	svc := NewPTVService(nil)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = svc.Prove(nil, "bench-agent", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))
		}
	})
}

func BenchmarkPTVProveAndCryptoVerify(b *testing.B) {
	svc := NewPTVService(nil)
	proof, _ := svc.Prove(nil, "bench-agent", "macos", "1.0", []byte("tpm-key"), []byte("runtime-hash"), []byte("nonce"))
	attBytes, _ := json.Marshal(proof.Attestation)
	hash := sha256.Sum256(attBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ecdsa.VerifyASN1(&svc.gatewayPrivKey.PublicKey, hash[:], proof.TPMSignature)
	}
}

func BenchmarkECDSASignP256(b *testing.B) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	msg := []byte("benchmark message for P-256 ECDSA signing performance")
	hash := sha256.Sum256(msg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ecdsa.SignASN1(rand.Reader, key, hash[:])
	}
}

func BenchmarkECDSAVerifyP256(b *testing.B) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	msg := []byte("benchmark message for P-256 ECDSA verification performance")
	hash := sha256.Sum256(msg)
	sig, _ := ecdsa.SignASN1(rand.Reader, key, hash[:])
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ecdsa.VerifyASN1(&key.PublicKey, hash[:], sig)
	}
}