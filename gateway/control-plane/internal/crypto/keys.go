package crypto

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadOrGenerateKeys(keyPath string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err == nil {
				if key, ok := parsed.(ed25519.PrivateKey); ok {
					pubKey := key.Public().(ed25519.PublicKey)
					return pubKey, key, nil
				}
			}
		}
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	if err := SaveKeys(pubKey, privKey, keyPath, keyPath+".pub"); err != nil {
		return nil, nil, fmt.Errorf("failed to save keys: %w", err)
	}

	return pubKey, privKey, nil
}

func SaveKeys(pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, keyPath, pubPath string) error {
	seed := privKey.Seed()
	privBytes, err := x509.MarshalPKCS8PrivateKey(ed25519.NewKeyFromSeed(seed))
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	privBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(privBlock), 0600); err != nil {
		return err
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(pubBlock), 0644); err != nil {
		return err
	}

	return nil
}