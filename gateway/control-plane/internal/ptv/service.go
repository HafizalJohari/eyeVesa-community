package ptv

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HardwareAttestation struct {
	AgentID         string `json:"agent_id"`
	Platform        string `json:"platform"`
	FirmwareVersion string `json:"firmware_version"`
	TPMPublicKey    []byte `json:"tpm_public_key"`
	RuntimeHash     []byte `json:"runtime_hash"`
	Timestamp       int64  `json:"timestamp"`
	Nonce           []byte `json:"nonce"`
}

type AttestationProof struct {
	Attestation  HardwareAttestation `json:"attestation"`
	TPMSignature []byte             `json:"tpm_signature"`
	Quote        []byte             `json:"quote"`
}

type IdentityBinding struct {
	BindingID        string `json:"binding_id"`
	AgentID          string `json:"agent_id"`
	AgentPublicKey   []byte `json:"agent_public_key"`
	HardwarePublicKey []byte `json:"hardware_public_key"`
	Platform         string `json:"platform"`
	RuntimeHash      []byte `json:"runtime_hash"`
	TransformedAt    int64  `json:"transformed_at"`
	BindingSignature []byte `json:"binding_signature"`
	ExpiresAt        int64  `json:"expires_at"`
}

type VerificationResult struct {
	Valid      bool   `json:"valid"`
	AgentID    string `json:"agent_id"`
	Platform   string `json:"platform"`
	Message    string `json:"message"`
	VerifiedAt int64  `json:"verified_at"`
}

type PTVService struct {
	db             *pgxpool.Pool
	gatewayPrivKey *ecdsa.PrivateKey
}

func NewPTVService(db *pgxpool.Pool) *PTVService {
	keyPath := os.Getenv("PTV_KEY_PATH")
	if keyPath == "" {
		keyPath = "/tmp/agentid-ptv-ecdsa.key"
	}

	var key *ecdsa.PrivateKey

	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err == nil {
				if k, ok := parsed.(*ecdsa.PrivateKey); ok {
					key = k
				}
			}
		}
	}

	if key == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(fmt.Sprintf("failed to generate PTV key: %v", err))
		}

		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err == nil {
			keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
			if writeErr := os.WriteFile(keyPath, keyPEM, 0600); writeErr != nil {
				log.Printf("[ptv] Warning: failed to write key to %s: %v", keyPath, writeErr)
			}
		}
	}

	return &PTVService{
		db:             db,
		gatewayPrivKey: key,
	}
}

func (s *PTVService) Prove(ctx context.Context, agentID, platform, firmwareVersion string, tpmPublicKey, runtimeHash, nonce []byte) (*AttestationProof, error) {
	attestation := HardwareAttestation{
		AgentID:         agentID,
		Platform:        platform,
		FirmwareVersion: firmwareVersion,
		TPMPublicKey:    tpmPublicKey,
		RuntimeHash:     runtimeHash,
		Timestamp:       time.Now().Unix(),
		Nonce:           nonce,
	}

	attBytes, err := json.Marshal(attestation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attestation: %w", err)
	}

	hash := sha256.Sum256(attBytes)
	sig, err := ecdsa.SignASN1(rand.Reader, s.gatewayPrivKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign attestation: %w", err)
	}

	quote := sha256.Sum256(append(attBytes, sig...))

	return &AttestationProof{
		Attestation:  attestation,
		TPMSignature: sig,
		Quote:        quote[:],
	}, nil
}

func (s *PTVService) Transform(ctx context.Context, proof *AttestationProof, agentPublicKey []byte) (*IdentityBinding, error) {
	attBytes, err := json.Marshal(proof.Attestation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attestation: %w", err)
	}

	hash := sha256.Sum256(attBytes)
	if !ecdsa.VerifyASN1(&s.gatewayPrivKey.PublicKey, hash[:], proof.TPMSignature) {
		return nil, fmt.Errorf("TPM signature verification failed")
	}

	bindingID := uuid.New().String()
	timestamp := time.Now().Unix()

	bindingData := fmt.Sprintf("%s:%s:%x:%x:%d",
		bindingID, proof.Attestation.AgentID,
		proof.Attestation.TPMPublicKey, proof.Attestation.RuntimeHash, timestamp)
	bindingHash := sha256.Sum256([]byte(bindingData))

	bindingSig, err := ecdsa.SignASN1(rand.Reader, s.gatewayPrivKey, bindingHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign binding: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO identity_bindings (binding_id, agent_id, platform, runtime_hash, hardware_public_key, binding_signature, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, to_timestamp($7))`,
		bindingID, proof.Attestation.AgentID, proof.Attestation.Platform,
		proof.Attestation.RuntimeHash, proof.Attestation.TPMPublicKey,
		bindingSig, timestamp+3600,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store binding: %w", err)
	}

	return &IdentityBinding{
		BindingID:         bindingID,
		AgentID:          proof.Attestation.AgentID,
		AgentPublicKey:   agentPublicKey,
		HardwarePublicKey: proof.Attestation.TPMPublicKey,
		Platform:         proof.Attestation.Platform,
		RuntimeHash:      proof.Attestation.RuntimeHash,
		TransformedAt:    timestamp,
		BindingSignature: bindingSig,
		ExpiresAt:        timestamp + 3600,
	}, nil
}

func (s *PTVService) Verify(ctx context.Context, bindingID string) (*VerificationResult, error) {
	var agentID, platform string
	var expiresAt time.Time
	err := s.db.QueryRow(ctx,
		`SELECT agent_id, platform, expires_at FROM identity_bindings WHERE binding_id = $1`,
		bindingID,
	).Scan(&agentID, &platform, &expiresAt)

	if err != nil {
		return &VerificationResult{
			Valid:      false,
			AgentID:    bindingID,
			Message:    "binding not found",
			VerifiedAt: time.Now().Unix(),
		}, nil
	}

	now := time.Now()
	valid := now.Before(expiresAt)

	return &VerificationResult{
		Valid:      valid,
		AgentID:    agentID,
		Platform:   platform,
		Message:    map[bool]string{true: "identity binding is valid", false: "binding has expired"}[valid],
		VerifiedAt: now.Unix(),
	}, nil
}