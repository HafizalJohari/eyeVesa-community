package crypto

import (
	"crypto/ed25519"
	"os"
	"testing"
)

func TestGenerateAgentKeypair(t *testing.T) {
	kp, err := GenerateAgentKeypair()
	if err != nil {
		t.Fatalf("GenerateAgentKeypair failed: %v", err)
	}
	if kp.PrivateKey == nil {
		t.Fatal("PrivateKey is nil")
	}
	if kp.PublicKey == nil {
		t.Fatal("PublicKey is nil")
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("PublicKey wrong size: got %d, want %d", len(kp.PublicKey), ed25519.PublicKeySize)
	}
}

func TestKeypairSignVerify(t *testing.T) {
	kp, _ := GenerateAgentKeypair()
	msg := []byte("test message for signing")
	sig := ed25519.Sign(kp.PrivateKey, msg)
	if !ed25519.Verify(kp.PublicKey, msg, sig) {
		t.Fatal("Signature verification failed")
	}
}

func TestEncodeDecodeBase64(t *testing.T) {
	data := []byte("hello world test data")
	encoded := EncodeBase64(data)
	decoded, err := DecodeBase64(encoded)
	if err != nil {
		t.Fatalf("DecodeBase64 failed: %v", err)
	}
	if string(decoded) != string(data) {
		t.Fatalf("Roundtrip failed: got %s, want %s", decoded, data)
	}
}

func TestVerifySignature(t *testing.T) {
	kp, _ := GenerateAgentKeypair()
	msg := []byte("verify this")
	sig := ed25519.Sign(kp.PrivateKey, msg)
	if !VerifySignature(kp.PublicKey, msg, sig) {
		t.Fatal("VerifySignature returned false for valid signature")
	}
}

func TestVerifySignatureInvalid(t *testing.T) {
	kp, _ := GenerateAgentKeypair()
	msg := []byte("verify this")
	sig := ed25519.Sign(kp.PrivateKey, msg)
	if VerifySignature(kp.PublicKey, []byte("wrong message"), sig) {
		t.Fatal("VerifySignature returned true for wrong message")
	}
}

func TestLoadOrGenerateKeysNew(t *testing.T) {
	keyPath := t.TempDir() + "/test.key"
	pubKey, privKey, err := LoadOrGenerateKeys(keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys failed: %v", err)
	}
	if pubKey == nil || privKey == nil {
		t.Fatal("Keys are nil")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("Key file was not created")
	}
}

func TestLoadOrGenerateKeysExisting(t *testing.T) {
	keyPath := t.TempDir() + "/test.key"
	pub1, priv1, _ := LoadOrGenerateKeys(keyPath)
	pub2, priv2, err := LoadOrGenerateKeys(keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateKeys (existing) failed: %v", err)
	}
	if string(pub1) != string(pub2) {
		t.Fatal("Public key changed on reload")
	}
	if string(priv1) != string(priv2) {
		t.Fatal("Private key changed on reload")
	}
}