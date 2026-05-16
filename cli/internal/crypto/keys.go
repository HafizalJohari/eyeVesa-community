package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

func SavePrivateKey(kp *KeyPair, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(kp.PrivateKey)
	return os.WriteFile(path, []byte(encoded), 0600)
}

func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(decoded))
	}
	return ed25519.PrivateKey(decoded), nil
}

func PublicKeyToBase64(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

func Base64ToPublicKey(s string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(decoded))
	}
	return ed25519.PublicKey(decoded), nil
}

func SavePublicKeyPem(kp *KeyPair, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(kp.PublicKey)
	return os.WriteFile(path, []byte(encoded), 0644)
}