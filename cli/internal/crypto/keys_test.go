package crypto

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	if len(kp.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("PrivateKey size = %d, want %d", len(kp.PrivateKey), ed25519.PrivateKeySize)
	}
	if len(kp.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("PublicKey size = %d, want %d", len(kp.PublicKey), ed25519.PublicKeySize)
	}
}

func TestSaveAndLoadPrivateKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.key")

	if err := SavePrivateKey(kp, path); err != nil {
		t.Fatalf("SavePrivateKey() error: %v", err)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey() error: %v", err)
	}

	if !equalKeys(loaded, kp.PrivateKey) {
		t.Error("loaded key does not match original")
	}
}

func TestLoadPrivateKeyInvalidSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	os.WriteFile(path, []byte("aGVsbG8="), 0600)

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Error("LoadPrivateKey() should fail for invalid key size")
	}
}

func TestPublicKeyToBase64(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	b64 := PublicKeyToBase64(kp.PublicKey)
	if b64 == "" {
		t.Error("PublicKeyToBase64() returned empty")
	}

	decoded, err := Base64ToPublicKey(b64)
	if err != nil {
		t.Fatalf("Base64ToPublicKey() error: %v", err)
	}

	if !equalPubKeys(decoded, kp.PublicKey) {
		t.Error("decoded public key does not match original")
	}
}

func TestBase64ToPublicKeyInvalid(t *testing.T) {
	_, err := Base64ToPublicKey("aW52YWxpZA==")
	if err == nil {
		t.Error("Base64ToPublicKey() should fail for invalid size")
	}
}

func equalKeys(a, b ed25519.PrivateKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalPubKeys(a, b ed25519.PublicKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}