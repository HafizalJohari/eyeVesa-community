package crypto

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type KeyRotationService struct {
	mu             sync.RWMutex
	currentPrivKey ed25519.PrivateKey
	currentPubKey  ed25519.PublicKey
	previousPrivKey ed25519.PrivateKey
	previousPubKey  ed25519.PublicKey
	rotatedAt      time.Time
	keyPath        string
	pubPath        string
	gracePeriod    time.Duration
}

func NewKeyRotationService(keyPath string, gracePeriod time.Duration) (*KeyRotationService, error) {
	pubKey, privKey, err := LoadOrGenerateKeys(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load keys: %w", err)
	}

	svc := &KeyRotationService{
		currentPrivKey: privKey,
		currentPubKey:  pubKey,
		keyPath:        keyPath,
		pubPath:        keyPath + ".pub",
		gracePeriod:    gracePeriod,
	}

	archivePath := keyPath + ".previous"
	if data, err := os.ReadFile(archivePath); err == nil {
		if block, _ := pem.Decode(data); block != nil {
			if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if key, ok := parsed.(ed25519.PrivateKey); ok {
					svc.previousPrivKey = key
					svc.previousPubKey = key.Public().(ed25519.PublicKey)
					slog.Info("loaded previous key for rotation grace period")
				}
			}
		}
	}

	return svc, nil
}

func (s *KeyRotationService) Sign(message []byte) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ed25519.Sign(s.currentPrivKey, message)
}

func (s *KeyRotationService) Verify(publicKey ed25519.PublicKey, message []byte, signature []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if ed25519.Verify(s.currentPubKey, message, signature) {
		return true
	}

	if len(s.previousPubKey) > 0 && s.isInGracePeriod() {
		if ed25519.Verify(s.previousPubKey, message, signature) {
			slog.Warn("verified with previous key (rotation grace period)", "grace_remaining", s.graceRemaining())
			return true
		}
	}

	return ed25519.Verify(publicKey, message, signature)
}

func (s *KeyRotationService) GetCurrentPublicKey() ed25519.PublicKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPubKey
}

func (s *KeyRotationService) GetCurrentPrivateKey() ed25519.PrivateKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPrivKey
}

func (s *KeyRotationService) Rotate() (ed25519.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.previousPrivKey = make(ed25519.PrivateKey, len(s.currentPrivKey))
	copy(s.previousPrivKey, s.currentPrivKey)
	s.previousPubKey = make(ed25519.PublicKey, len(s.currentPubKey))
	copy(s.previousPubKey, s.currentPubKey)

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("generate new key: %w", err)
	}

	archivePath := s.keyPath + ".previous"
	if err := SaveKeys(s.previousPubKey, s.previousPrivKey, archivePath, archivePath+".pub"); err != nil {
		slog.Warn("failed to archive previous key", "error", err)
	}

	tmpKeyPath := s.keyPath + ".tmp"
	tmpPubPath := s.pubPath + ".tmp"
	if err := SaveKeys(pubKey, privKey, tmpKeyPath, tmpPubPath); err != nil {
		return nil, fmt.Errorf("save new key to temp: %w", err)
	}

	if err := os.Rename(tmpKeyPath, s.keyPath); err != nil {
		return nil, fmt.Errorf("rename key file: %w", err)
	}
	if err := os.Rename(tmpPubPath, s.pubPath); err != nil {
		slog.Warn("failed to rename public key file", "error", err)
	}

	s.currentPrivKey = privKey
	s.currentPubKey = pubKey
	s.rotatedAt = time.Now()

	slog.Info("key rotation completed",
		"key_path", s.keyPath,
		"grace_period", s.gracePeriod,
		"public_key_b64", EncodeBase64(pubKey),
	)

	return pubKey, nil
}

func (s *KeyRotationService) GetRotationStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"has_previous_key": len(s.previousPubKey) > 0,
		"current_public":  EncodeBase64(s.currentPubKey),
		"grace_period":     s.gracePeriod.String(),
	}

	if len(s.previousPubKey) > 0 {
		status["previous_public"] = EncodeBase64(s.previousPubKey)
	}

	if !s.rotatedAt.IsZero() {
		status["last_rotated_at"] = s.rotatedAt.Format(time.RFC3339)
		status["grace_remaining"] = s.graceRemaining().String()
	}

	return status
}

func (s *KeyRotationService) isInGracePeriod() bool {
	if s.rotatedAt.IsZero() {
		return len(s.previousPubKey) > 0
	}
	return time.Since(s.rotatedAt) < s.gracePeriod
}

func (s *KeyRotationService) graceRemaining() time.Duration {
	if s.rotatedAt.IsZero() {
		return s.gracePeriod
	}
	remaining := s.gracePeriod - time.Since(s.rotatedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *KeyRotationService) ClearPreviousKey() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previousPrivKey = nil
	s.previousPubKey = nil
	slog.Info("previous key cleared, grace period ended")
}

func (s *KeyRotationService) StartAutoRotation(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := s.Rotate(); err != nil {
					slog.Error("auto key rotation failed", "error", err)
				}
			case <-stop:
				return
			}
		}
	}()
	return stop
}

func DefaultKeyPaths() (keyPath, ptvKeyPath string) {
	keyPath = os.Getenv("GATEWAY_KEY_PATH")
	if keyPath == "" {
		keyPath = "/tmp/agentid-gateway-ed25519.key"
	}
	ptvKeyPath = os.Getenv("PTV_KEY_PATH")
	if ptvKeyPath == "" {
		ptvKeyPath = "/tmp/agentid-gateway-ptv-ecdsa.key"
	}
	return keyPath, ptvKeyPath
}

func EnsureKeyDir(keyPath string) error {
	dir := filepath.Dir(keyPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0700)
}