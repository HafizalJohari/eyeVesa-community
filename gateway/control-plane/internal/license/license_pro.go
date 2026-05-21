//go:build pro

package license

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type LicenseClaims struct {
	Tier         Tier     `json:"tier"`
	MaxAgents    int      `json:"max_agents"`
	MaxResources int      `json:"max_resources"`
	Customer     string   `json:"customer"`
	IssuedAt     string   `json:"issued_at"`
	ExpiresAt    string   `json:"expires_at"`
	Features     []string `json:"features"`
	Signature    string   `json:"signature"`
}

var (
	publicKey    []byte
	loadKeyOnce  sync.Once
	// BakedPublicKey can be injected at compile time via:
	// -ldflags "-X github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/license.BakedPublicKey=hexstring"
	BakedPublicKey string
)

var proFeatures = []string{
	FeatureMultiTenant,
	FeatureMultiLayerHITL,
	FeatureSlackNotify,
	FeaturePagerDuty,
	FeatureSSO,
	FeatureLLM,
	FeatureAnomalyDetect,
	FeatureBudget,
	FeatureRateLimit,
	FeatureKubernetes,
	FeatureDelegation,
	FeaturePushNotify,
	FeatureFederation,
}

func getPublicKey() []byte {
	loadKeyOnce.Do(func() {
		keyHex := BakedPublicKey
		if keyHex == "" {
			keyHex = os.Getenv("EYEVESA_PUBLIC_KEY")
		}
		if keyHex == "" {
			fmt.Fprintf(os.Stderr, "FATAL: EYEVESA_PUBLIC_KEY environment variable or BakedPublicKey is required\n")
			os.Exit(1)
		}
		var err error
		publicKey, err = hex.DecodeString(keyHex)
		if err != nil || len(publicKey) != ed25519.PublicKeySize {
			fmt.Fprintf(os.Stderr, "FATAL: invalid public key: must be a %d-byte Ed25519 public key hex string\n", ed25519.PublicKeySize)
			os.Exit(1)
		}
	})
	return publicKey
}

func Load() Info {
	// Support both EYEVESA_LICENSE_FILE (preferred) and EYEVESA_LICENSE_KEY (legacy).
	key := os.Getenv("EYEVESA_LICENSE_FILE")
	if key == "" {
		key = os.Getenv("EYEVESA_LICENSE_KEY")
	}
	if key == "" {
		return Info{
			Tier:         TierCommunity,
			MaxAgents:    5,
			MaxResources: 10,
			Features: []string{
				FeatureDelegation,
				FeatureFederation,
			},
		}
	}

	claims, err := decodeAndVerify(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: invalid license key: %v (falling back to Community)\n", err)
		return Info{
			Tier:         TierCommunity,
			MaxAgents:    5,
			MaxResources: 10,
			Features: []string{
				FeatureDelegation,
				FeatureFederation,
			},
		}
	}

	expires, err := time.Parse(time.RFC3339, claims.ExpiresAt)
	if err == nil && time.Now().After(expires) {
		fmt.Fprintf(os.Stderr, "WARNING: license expired at %s (falling back to Community)\n", claims.ExpiresAt)
		return Info{
			Tier:         TierCommunity,
			MaxAgents:    5,
			MaxResources: 10,
			Features: []string{
				FeatureDelegation,
				FeatureFederation,
			},
		}
	}

	return Info{
		Tier:         claims.Tier,
		MaxAgents:    claims.MaxAgents,
		MaxResources: claims.MaxResources,
		Features:     claims.Features,
	}
}

func Validate(key string) error {
	_, err := decodeAndVerify(key)
	return err
}

func decodeAndVerify(key string) (*LicenseClaims, error) {
	data, err := os.ReadFile(key)
	if err != nil {
		return nil, fmt.Errorf("read license file: %w", err)
	}

	var claims LicenseClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	sig, err := hex.DecodeString(claims.Signature)
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	claims.Signature = ""

	payload, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	if !ed25519.Verify(getPublicKey(), payload, sig) {
		return nil, fmt.Errorf("invalid signature")
	}

	return &claims, nil
}
