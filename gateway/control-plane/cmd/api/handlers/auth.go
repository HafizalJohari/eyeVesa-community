package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/auth"
)

var (
	jwtSecret []byte
	challengesMu sync.RWMutex
	challenges = make(map[string]challengeEntry)
)

func SetJWTSecret(secret string) {
	jwtSecret = []byte(secret)
}

func getJWTSecret() []byte {
	return jwtSecret
}

func generateAPIKey() string {
	return auth.GenerateAPIKey()
}

func hashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

func createAPIKeyForTenant(ctx context.Context, name, tenantID string) (APIKeyResponse, error) {
	apiKey := generateAPIKey()
	keyID := uuid.New()

	var createdAt time.Time
	var tenantArg *string
	if tenantID != "" {
		tenantArg = &tenantID
	}

	err := querier.QueryRow(ctx,
		`INSERT INTO api_keys (key_id, api_key_hash, name, tenant_id, is_active, created_at)
		 VALUES ($1, $2, $3, $4, true, NOW()) RETURNING created_at`,
		keyID, hashAPIKey(apiKey), name, tenantArg,
	).Scan(&createdAt)
	if err != nil {
		return APIKeyResponse{}, err
	}

	return APIKeyResponse{
		KeyID:     keyID.String(),
		APIKey:    apiKey,
		Name:      name,
		TenantID:  tenantID,
		CreatedAt: createdAt,
	}, nil
}

func generateJWT(secret []byte, claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

type CreateAPIKeyRequest struct {
	Name     string `json:"name"`
	TenantID string `json:"tenant_id,omitempty"`
}

type APIKeyResponse struct {
	KeyID     string    `json:"key_id"`
	APIKey    string    `json:"api_key"`
	Name      string    `json:"name"`
	TenantID  string    `json:"tenant_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	resp, err := createAPIKeyForTenant(r.Context(), req.Name, req.TenantID)
	if err != nil {
		slog.Error("create api key failed", "error", err)
		http.Error(w, "failed to create api key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.GetTenantID(r.Context())
	query := `SELECT key_id, name, tenant_id, is_active, created_at FROM api_keys`
	args := []interface{}{}
	if tenantID != "" {
		query += ` WHERE tenant_id = $1`
		args = append(args, tenantID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := querier.Query(r.Context(), query, args...)
	if err != nil {
		slog.Error("list api keys failed", "error", err)
		http.Error(w, "failed to list api keys", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type keyEntry struct {
		KeyID     string    `json:"key_id"`
		Name      string    `json:"name"`
		TenantID  string    `json:"tenant_id,omitempty"`
		IsActive  bool      `json:"is_active"`
		CreatedAt time.Time `json:"created_at"`
	}

	keys := []keyEntry{}
	for rows.Next() {
		var k keyEntry
		var tid *string
		err := rows.Scan(&k.KeyID, &k.Name, &tid, &k.IsActive, &k.CreatedAt)
		if err != nil {
			continue
		}
		if tid != nil {
			k.TenantID = *tid
		}
		keys = append(keys, k)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keys":  keys,
		"count": len(keys),
	})
}

func RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if keyID == "" {
		http.Error(w, "key_id required", http.StatusBadRequest)
		return
	}

	tenantID := auth.GetTenantID(r.Context())
	query := `UPDATE api_keys SET is_active = false WHERE key_id = $1`
	args := []interface{}{keyID}
	if tenantID != "" {
		query += ` AND tenant_id = $2`
		args = append(args, tenantID)
	}
	tag, err := querier.Exec(r.Context(), query, args...)
	if err != nil {
		slog.Error("revoke api key failed", "error", err)
		http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected == 0 {
		http.Error(w, "api key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key_id":  keyID,
		"revoked": true,
	})
}

type ChallengeRequest struct {
	AgentID string `json:"agent_id"`
}

type ChallengeResponse struct {
	AgentID   string `json:"agent_id"`
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"expires_at"`
}

type challengeEntry struct {
	nonce     string
	expiresAt time.Time
}

func AuthChallenge(w http.ResponseWriter, r *http.Request) {
	var req ChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	var status string
	err = querier.QueryRow(r.Context(),
		`SELECT status FROM agents WHERE agent_id = $1`, agentID,
	).Scan(&status)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	if status != "active" {
		http.Error(w, "agent is not active", http.StatusForbidden)
		return
	}

	nonce := uuid.New().String()
	expiresAt := time.Now().Add(5 * time.Minute)

	challengesMu.Lock()
	challenges[agentID.String()] = challengeEntry{
		nonce:     nonce,
		expiresAt: expiresAt,
	}
	challengesMu.Unlock()

	resp := ChallengeResponse{
		AgentID:   agentID.String(),
		Nonce:     nonce,
		ExpiresAt: expiresAt.Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type LoginRequest struct {
	AgentID   string `json:"agent_id"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	AgentID   string    `json:"agent_id"`
}

func AgentLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Nonce == "" || req.Signature == "" {
		http.Error(w, "agent_id, nonce, and signature are required", http.StatusBadRequest)
		return
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		http.Error(w, "invalid agent_id", http.StatusBadRequest)
		return
	}

	challengesMu.RLock()
	challenge, exists := challenges[agentID.String()]
	challengesMu.RUnlock()

	if !exists {
		http.Error(w, "no challenge found — request a challenge first via POST /v1/auth/challenge", http.StatusBadRequest)
		return
	}

	if time.Now().After(challenge.expiresAt) {
		challengesMu.Lock()
		delete(challenges, agentID.String())
		challengesMu.Unlock()
		http.Error(w, "challenge expired — request a new challenge", http.StatusBadRequest)
		return
	}

	if challenge.nonce != req.Nonce {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}

	var publicKeyBytes []byte
	err = querier.QueryRow(r.Context(),
		`SELECT public_key FROM agents WHERE agent_id = $1`, agentID,
	).Scan(&publicKeyBytes)
	if err != nil {
		slog.Error("agent login failed - public key not found", "agent_id", agentID, "error", err)
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		http.Error(w, "invalid signature: must be base64-encoded", http.StatusBadRequest)
		return
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		slog.Error("invalid public key length", "agent_id", agentID, "len", len(publicKeyBytes))
		http.Error(w, "invalid agent public key", http.StatusInternalServerError)
		return
	}

	message := []byte(challenge.nonce)
	if !ed25519.Verify(publicKeyBytes, message, signatureBytes) {
		http.Error(w, "signature verification failed — you do not own this identity", http.StatusUnauthorized)
		return
	}

	challengesMu.Lock()
	delete(challenges, agentID.String())
	challengesMu.Unlock()

	secret := getJWTSecret()
	if len(secret) == 0 {
		slog.Error("jwt secret not configured")
		http.Error(w, "authentication not configured", http.StatusInternalServerError)
		return
	}

	tokenExpiresAt := time.Now().Add(24 * time.Hour)
	token, err := generateJWT(secret, jwt.MapClaims{
		"agent_id": agentID.String(),
		"role":     "agent",
		"exp":      tokenExpiresAt.Unix(),
		"iat":      time.Now().Unix(),
	})
	if err != nil {
		slog.Error("generate jwt failed", "error", err)
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	if auditLogger != nil && gatewayPrivateKey != nil {
		auditEntry := audit.AuditEntry{
			AgentID: agentID.String(),
			Action:  "agent.login",
			Method:  "HTTP",
			Status:  "success",
		}
		auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)
	}

	resp := LoginResponse{
		Token:     token,
		ExpiresAt: tokenExpiresAt,
		AgentID:   agentID.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
