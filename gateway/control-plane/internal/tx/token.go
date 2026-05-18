package tx

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CapabilityToken struct {
	ID          string                 `json:"jti"`
	Issuer      string                 `json:"iss"`
	Subject     string                 `json:"sub"`
	ResourceID  string                 `json:"resource_id"`
	Action      string                 `json:"action"`
	Scopes      []string               `json:"scopes"`
	TrustScore  float64                `json:"trust_score"`
	AgentSkills []SkillClaim           `json:"skills,omitempty"`
	Params      map[string]interface{}  `json:"params,omitempty"`
	IssuedAt    int64                  `json:"iat"`
	ExpiresAt   int64                  `json:"exp"`
	Nonce      string                 `json:"nonce"`
	Signature  string                 `json:"sig,omitempty"`
}

type SkillClaim struct {
	SkillID     string `json:"skill_id"`
	SkillName   string `json:"skill_name"`
	Proficiency int   `json:"proficiency"`
	Verified    bool   `json:"verified"`
}

type TransactionReceipt struct {
	ReceiptID     string          `json:"receipt_id"`
	TokenID       string          `json:"token_id"`
	AgentID       string          `json:"agent_id"`
	ResourceID    string          `json:"resource_id"`
	Action        string          `json:"action"`
	Allowed       bool            `json:"allowed"`
	TrustScore    float64         `json:"trust_score"`
	TrustDelta    float64         `json:"trust_delta"`
	TokenIssuedAt int64           `json:"token_issued_at"`
	TokenExpires  int64           `json:"token_expires"`
	IssuedAt      time.Time       `json:"issued_at"`
	Signature     string          `json:"signature,omitempty"`
}

type TokenService struct {
	privateKey   ed25519.PrivateKey
	publicKey    ed25519.PublicKey
	tokenExpiry  time.Duration
}

func NewTokenService(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey, tokenExpiry time.Duration) *TokenService {
	if tokenExpiry == 0 {
		tokenExpiry = 5 * time.Minute
	}
	return &TokenService{
		privateKey:  privateKey,
		publicKey:   publicKey,
		tokenExpiry: tokenExpiry,
	}
}

func (s *TokenService) IssueToken(agentID, resourceID, action string, trustScore float64, scopes []string, skills []SkillClaim, params map[string]interface{}) (*CapabilityToken, error) {
	now := time.Now()
	token := &CapabilityToken{
		ID:         uuid.New().String(),
		Issuer:     "agentid-gateway",
		Subject:    agentID,
		ResourceID: resourceID,
		Action:     action,
		Scopes:     scopes,
		TrustScore: trustScore,
		AgentSkills: skills,
		Params:     params,
		IssuedAt:   now.Unix(),
		ExpiresAt:  now.Add(s.tokenExpiry).Unix(),
		Nonce:      uuid.New().String()[:12],
	}

	sig, err := s.signToken(token)
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}
	token.Signature = sig

	return token, nil
}

func (s *TokenService) VerifyToken(token *CapabilityToken) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}

	now := time.Now().Unix()
	if now > token.ExpiresAt {
		return fmt.Errorf("token expired at %d", token.ExpiresAt)
	}

	if token.Signature == "" {
		return fmt.Errorf("token has no signature")
	}

	savedSig := token.Signature
	token.Signature = ""

	payload, err := json.Marshal(token)
	if err != nil {
		token.Signature = savedSig
		return fmt.Errorf("marshal token for verification: %w", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(savedSig)
	if err != nil {
		token.Signature = savedSig
		return fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(s.publicKey, payload, sigBytes) {
		token.Signature = savedSig
		return fmt.Errorf("invalid signature")
	}

	token.Signature = savedSig
	return nil
}

func (s *TokenService) DecodeAndVerifyToken(tokenJSON string) (*CapabilityToken, error) {
	var token CapabilityToken
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	if err := s.VerifyToken(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (s *TokenService) IssueReceipt(token *CapabilityToken, allowed bool, trustScore, trustDelta float64) (*TransactionReceipt, error) {
	now := time.Now()
	receipt := &TransactionReceipt{
		ReceiptID:     uuid.New().String(),
		TokenID:       token.ID,
		AgentID:       token.Subject,
		ResourceID:    token.ResourceID,
		Action:        token.Action,
		Allowed:       allowed,
		TrustScore:    trustScore,
		TrustDelta:    trustDelta,
		TokenIssuedAt: token.IssuedAt,
		TokenExpires:  token.ExpiresAt,
		IssuedAt:      now,
	}

	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		return nil, fmt.Errorf("marshal receipt: %w", err)
	}

	sig := ed25519.Sign(s.privateKey, receiptBytes)
	receipt.Signature = base64.StdEncoding.EncodeToString(sig)

	return receipt, nil
}

func (s *TokenService) VerifyReceipt(receipt *TransactionReceipt) error {
	if receipt == nil {
		return fmt.Errorf("receipt is nil")
	}

	if receipt.Signature == "" {
		return fmt.Errorf("receipt has no signature")
	}

	savedSig := receipt.Signature
	receipt.Signature = ""

	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		receipt.Signature = savedSig
		return fmt.Errorf("marshal receipt for verification: %w", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(savedSig)
	if err != nil {
		receipt.Signature = savedSig
		return fmt.Errorf("decode signature: %w", err)
	}

	if !ed25519.Verify(s.publicKey, receiptBytes, sigBytes) {
		receipt.Signature = savedSig
		return fmt.Errorf("invalid receipt signature")
	}

	receipt.Signature = savedSig
	return nil
}

func (s *TokenService) signToken(token *CapabilityToken) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("no private key configured")
	}

	payload, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("marshal token: %w", err)
	}

	sig := ed25519.Sign(s.privateKey, payload)
	return base64.StdEncoding.EncodeToString(sig), nil
}

func (s *TokenService) TokenExpiry() time.Duration {
	return s.tokenExpiry
}