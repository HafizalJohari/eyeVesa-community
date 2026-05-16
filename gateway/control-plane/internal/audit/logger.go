package audit

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type AuditEntry struct {
	LogID      string                 `json:"log_id"`
	AgentID    string                 `json:"agent_id"`
	ResourceID string                `json:"resource_id"`
	Action     string                 `json:"action"`
	Method     string                 `json:"method"`
	Params     map[string]interface{} `json:"params"`
	Result     map[string]interface{} `json:"result"`
	Status     string                 `json:"result_status"`
	TrustBefore float64               `json:"trust_score_before"`
	TrustAfter  float64               `json:"trust_score_after"`
	SessionID  string                 `json:"session_id"`
}

type AuditLogger struct {
	db *database.DB
}

func NewAuditLogger(db *database.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

func (a *AuditLogger) Log(ctx context.Context, entry AuditEntry, signingKey ed25519.PrivateKey) error {
	if entry.LogID == "" {
		entry.LogID = uuid.New().String()
	}

	paramsJSON, _ := json.Marshal(entry.Params)
	resultJSON, _ := json.Marshal(entry.Result)

	var signature []byte
	if signingKey != nil {
		sig, sigErr := a.computeSignature(entry, signingKey)
		if sigErr != nil {
			return fmt.Errorf("failed to compute signature: %w", sigErr)
		}
		signature = sig
	} else {
		log.Printf("[audit] Log %s: no signing key provided, entry stored without signature", entry.LogID)
	}

	_, err := a.db.Pool.Exec(ctx,
		`INSERT INTO audit_logs (log_id, agent_id, resource_id, action, method, params, result, result_status, trust_score_before, trust_score_after, session_id, signature)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		entry.LogID, entry.AgentID, nilIfEmpty(entry.ResourceID), entry.Action, entry.Method,
		paramsJSON, resultJSON, entry.Status,
		entry.TrustBefore, entry.TrustAfter,
		nilIfEmpty(entry.SessionID), signature,
	)

	return err
}

func (a *AuditLogger) computeSignature(entry AuditEntry, key ed25519.PrivateKey) ([]byte, error) {
	payload := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		entry.LogID, entry.AgentID, entry.ResourceID,
		entry.Action, entry.Method, entry.Status)

	hash := sha256.Sum256([]byte(payload))
	sig := ed25519.Sign(key, hash[:])
	return sig, nil
}

func (a *AuditLogger) VerifyIntegrity(ctx context.Context, logID string, publicKey ed25519.PublicKey) (bool, error) {
	var action, method, status, agentID, resourceID string
	var signature []byte

	err := a.db.Pool.QueryRow(ctx,
		`SELECT agent_id, resource_id, action, method, result_status, signature FROM audit_logs WHERE log_id = $1`,
		logID,
	).Scan(&agentID, &resourceID, &action, &method, &status, &signature)

	if err != nil {
		return false, err
	}

	payload := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		logID, agentID, resourceID, action, method, status)

	hash := sha256.Sum256([]byte(payload))
	return ed25519.Verify(publicKey, hash[:], signature), nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func ComputeLogHash(entries []AuditEntry) string {
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.LogID))
		h.Write([]byte(e.AgentID))
		h.Write([]byte(e.Action))
		h.Write([]byte(e.Status))
	}
	return hex.EncodeToString(h.Sum(nil))
}