package crypto

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyRotationService_New(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, err := NewKeyRotationService(keyPath, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewKeyRotationService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("service should not be nil")
	}
	if len(svc.currentPubKey) == 0 {
		t.Fatal("should have current public key")
	}
	if len(svc.currentPrivKey) == 0 {
		t.Fatal("should have current private key")
	}
}

func TestKeyRotationService_SignVerify(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)

	message := []byte("test message for signing")
	sig := svc.Sign(message)
	if len(sig) == 0 {
		t.Fatal("signature should not be empty")
	}

	if !ed25519.Verify(svc.GetCurrentPublicKey(), message, sig) {
		t.Fatal("signature should verify with current public key")
	}
}

func TestKeyRotationService_Rotate(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	oldPub := svc.GetCurrentPublicKey()

	message := []byte("before rotation")
	sig := svc.Sign(message)

	newPub, err := svc.Rotate()
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	if string(newPub) == string(oldPub) {
		t.Fatal("new public key should differ from old")
	}

	if !ed25519.Verify(oldPub, message, sig) {
		t.Fatal("old signature should still verify with old public key")
	}

	message2 := []byte("after rotation")
	sig2 := svc.Sign(message2)
	if !ed25519.Verify(newPub, message2, sig2) {
		t.Fatal("new signature should verify with new public key")
	}

	if ed25519.Verify(oldPub, message2, sig2) {
		t.Fatal("new signature should NOT verify with old public key")
	}
}

func TestKeyRotationService_GracePeriod(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, _ := NewKeyRotationService(keyPath, 10*time.Minute)

	message := []byte("grace period test")
	sig := svc.Sign(message)

	svc.Rotate()

	if !svc.Verify(svc.GetCurrentPublicKey(), message, sig) {
		t.Fatal("current key should verify current signatures")
	}
}

func TestKeyRotationService_ClearPreviousKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	svc.Rotate()

	status := svc.GetRotationStatus()
	hasPrev, _ := status["has_previous_key"].(bool)
	if !hasPrev {
		t.Fatal("should have previous key after rotation")
	}

	svc.ClearPreviousKey()

	status = svc.GetRotationStatus()
	hasPrev, _ = status["has_previous_key"].(bool)
	if hasPrev {
		t.Fatal("should not have previous key after clear")
	}
}

func TestKeyRotationService_RotationStatus(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test.key")

	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	status := svc.GetRotationStatus()

	if status["current_public"] == nil {
		t.Fatal("status should have current_public")
	}
	if status["grace_period"] == nil {
		t.Fatal("status should have grace_period")
	}

	svc.Rotate()

	status = svc.GetRotationStatus()
	if status["last_rotated_at"] == nil {
		t.Fatal("status should have last_rotated_at after rotation")
	}
}

func TestKeyRotationService_FilePersistence(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "persist.key")

	svc1, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	pub1 := svc1.GetCurrentPublicKey()
	sig1 := svc1.Sign([]byte("persist test"))

	svc2, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	pub2 := svc2.GetCurrentPublicKey()

	if string(pub1) != string(pub2) {
		t.Fatal("key should persist across service restarts")
	}

	if !ed25519.Verify(pub2, []byte("persist test"), sig1) {
		t.Fatal("signature from first instance should verify with key from second")
	}
}

func TestKeyRotationService_ArchivePreviousKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "archive.key")

	svc, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	oldPub := svc.GetCurrentPublicKey()
	svc.Rotate()

	archivePath := keyPath + ".previous"
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("previous key should be archived")
	}

	svc2, _ := NewKeyRotationService(keyPath, 5*time.Minute)
	if len(svc2.previousPubKey) == 0 {
		t.Fatal("second instance should load previous key")
	}

	if string(svc2.previousPubKey) != string(oldPub) {
		t.Fatal("previous key should match original key")
	}
}

func TestDefaultKeyPaths(t *testing.T) {
	keyPath, ptvKeyPath := DefaultKeyPaths()
	if keyPath == "" {
		t.Fatal("key path should not be empty")
	}
	if ptvKeyPath == "" {
		t.Fatal("ptv key path should not be empty")
	}

	os.Setenv("GATEWAY_KEY_PATH", "/custom/key")
	defer os.Unsetenv("GATEWAY_KEY_PATH")
	customKey, _ := DefaultKeyPaths()
	if customKey != "/custom/key" {
		t.Fatalf("expected /custom/key, got %s", customKey)
	}
}

func TestEnsureKeyDir(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "subdir", "nested", "key.key")

	if err := EnsureKeyDir(keyPath); err != nil {
		t.Fatalf("EnsureKeyDir failed: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(keyPath)); os.IsNotExist(err) {
		t.Fatal("directory should exist after EnsureKeyDir")
	}
}