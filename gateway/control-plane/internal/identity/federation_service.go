package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type FederationPeer struct {
	GatewayID    string  `json:"gateway_id"`
	Name         string  `json:"name"`
	PublicKey    string  `json:"public_key"`
	Endpoint     string  `json:"endpoint"`
	TrustDomain  string  `json:"trust_domain"`
	PeerType     string  `json:"peer_type"`
	Status       string  `json:"status"`
	TrustScore   float64 `json:"trust_score"`
	AgentCount   int     `json:"agent_count"`
	LastSyncAt   string  `json:"last_sync_at,omitempty"`
	RegisteredAt string  `json:"registered_at"`
}

type FederationPeerInvite struct {
	InviteID    string `json:"invite_id"`
	Token       string `json:"token,omitempty"`
	TokenHash   string `json:"-"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	TrustDomain string `json:"trust_domain"`
	PeerType    string `json:"peer_type"`
	ExpiresAt   string `json:"expires_at"`
	UsedAt      string `json:"used_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type FederatedAgent struct {
	AgentID                  string   `json:"agent_id"`
	GatewayID                string   `json:"gateway_id"`
	Name                     string   `json:"name"`
	Owner                    string   `json:"owner"`
	PublicKey                string   `json:"public_key"`
	TrustScore               float64  `json:"trust_score"`
	Capabilities             []string `json:"capabilities"`
	AllowedTools             []string `json:"allowed_tools"`
	PassportSignature        string   `json:"passport_signature"`
	PassportIssuedAt         string   `json:"passport_issued_at"`
	Scope                    string   `json:"scope"`
	Status                   string   `json:"status"`
	Description              string   `json:"description,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	HeartbeatStatus          string   `json:"heartbeat_status,omitempty"`
	LastHeartbeat            string   `json:"last_heartbeat,omitempty"`
	MerchantTrustScore       float64  `json:"merchant_trust_score,omitempty"`
	MerchantConfidence       float64  `json:"merchant_confidence,omitempty"`
	MerchantVerificationTier string   `json:"merchant_verification_tier,omitempty"`
	MerchantRiskFlags        []string `json:"merchant_risk_flags,omitempty"`
	MerchantHITLOnly         bool     `json:"merchant_hitl_only,omitempty"`
	MerchantSuspended        bool     `json:"merchant_suspended,omitempty"`
}

type FederatedMerchantTrustSync struct {
	MerchantID       string                 `json:"merchant_id"`
	GatewayID        string                 `json:"gateway_id"`
	TrustScore       float64                `json:"trust_score"`
	Confidence       float64                `json:"confidence"`
	VerificationTier string                 `json:"verification_tier"`
	RiskFlags        []string               `json:"risk_flags"`
	HITLOnly         bool                   `json:"hitl_only"`
	Suspended        bool                   `json:"suspended"`
	OrderCount       int                    `json:"order_count"`
	OrderID          string                 `json:"order_id"`
	OutcomeStatus    string                 `json:"outcome_status"`
	DisputeRef       string                 `json:"dispute_ref"`
	ReceiptSignature string                 `json:"receipt_signature"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type FederatedInvokeRequest struct {
	RequesterID string                 `json:"requester_id"`
	ResponderID string                 `json:"responder_id"`
	Action      string                 `json:"action"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

type FederatedInvokeDecision struct {
	Allowed            bool    `json:"allowed"`
	RequiresHITL       bool    `json:"requires_hitl"`
	Reason             string  `json:"reason"`
	RiskLevel          string  `json:"risk_level"`
	RequesterTrust     float64 `json:"requester_trust"`
	ResponderTrust     float64 `json:"responder_trust"`
	ResponderGatewayID string  `json:"responder_gateway_id"`
	ExecutionEnabled   bool    `json:"execution_enabled"`
}

type AgentPassport struct {
	AgentID     string `json:"agent_id"`
	AgentPubKey string `json:"agent_public_key"`
	GatewayID   string `json:"gateway_id"`
	GatewaySig  string `json:"gateway_signature"`
	IssuedAt    string `json:"issued_at"`
}

type FederationService struct {
	pool    *pgxpool.Pool
	querier database.Querier
}

func NewFederationService(pool *pgxpool.Pool) *FederationService {
	return &FederationService{
		pool:    pool,
		querier: &database.PoolQuerier{Pool: pool},
	}
}

func (fs *FederationService) SetQuerierForTest(q database.Querier) {
	fs.querier = q
}

func (fs *FederationService) CreatePeerInvite(ctx context.Context, name, endpoint, trustDomain, peerType string, ttl time.Duration) (*FederationPeerInvite, error) {
	if name == "" || endpoint == "" {
		return nil, fmt.Errorf("name and endpoint are required")
	}
	if err := validateFederationEndpoint(endpoint); err != nil {
		return nil, err
	}
	peerType = normalizePeerType(peerType)
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if trustDomain == "" {
		trustDomain = name
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate invite token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := hashInviteToken(token)

	var inviteID string
	var expiresAt, createdAt time.Time
	err := fs.querier.QueryRow(ctx,
		`INSERT INTO federation_peer_invites (token_hash, name, endpoint, trust_domain, peer_type, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW() + ($6::int * INTERVAL '1 second'), NOW())
		 RETURNING invite_id, expires_at, created_at`,
		tokenHash, name, endpoint, trustDomain, peerType, int(ttl.Seconds()),
	).Scan(&inviteID, &expiresAt, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("create federation peer invite failed: %w", err)
	}

	return &FederationPeerInvite{
		InviteID:    inviteID,
		Token:       token,
		TokenHash:   tokenHash,
		Name:        name,
		Endpoint:    endpoint,
		TrustDomain: trustDomain,
		PeerType:    peerType,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		CreatedAt:   createdAt.Format(time.RFC3339),
	}, nil
}

func (fs *FederationService) RegisterPeer(ctx context.Context, name, publicKeyB64, endpoint, trustDomain, peerType, inviteToken string, adminApproved bool) (*FederationPeer, error) {
	if err := validateFederationEndpoint(endpoint); err != nil {
		return nil, err
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid public_key base64: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public_key: must be %d bytes, got %d", ed25519.PublicKeySize, len(pubKeyBytes))
	}

	peerType = normalizePeerType(peerType)
	inviteID := ""
	if !adminApproved {
		if inviteToken == "" {
			return nil, fmt.Errorf("invite_token is required")
		}
		var err error
		inviteID, err = fs.validatePeerInvite(ctx, inviteToken, name, endpoint, trustDomain, peerType)
		if err != nil {
			return nil, err
		}
	}

	var gatewayID string
	var registeredAt time.Time
	err = fs.querier.QueryRow(ctx,
		`INSERT INTO federation_peers (name, public_key, endpoint, trust_domain, peer_type, status, trust_score, registered_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'active', 1.0, NOW(), NOW())
		 ON CONFLICT (endpoint) DO UPDATE SET name = EXCLUDED.name, public_key = EXCLUDED.public_key, trust_domain = EXCLUDED.trust_domain, peer_type = EXCLUDED.peer_type, status = 'active', updated_at = NOW()
		 RETURNING gateway_id, registered_at`,
		name, pubKeyBytes, endpoint, trustDomain, peerType,
	).Scan(&gatewayID, &registeredAt)

	if err != nil {
		return nil, fmt.Errorf("failed to register peer: %w", err)
	}
	if inviteID != "" {
		if err := fs.markPeerInviteUsed(ctx, inviteID); err != nil {
			return nil, err
		}
	}

	slog.Info("federation peer registered", "gateway_id", gatewayID, "name", name, "endpoint", endpoint, "peer_type", peerType)

	return &FederationPeer{
		GatewayID:    gatewayID,
		Name:         name,
		PublicKey:    publicKeyB64,
		Endpoint:     endpoint,
		TrustDomain:  trustDomain,
		PeerType:     peerType,
		Status:       "active",
		TrustScore:   1.0,
		RegisteredAt: registeredAt.Format(time.RFC3339),
	}, nil
}

func (fs *FederationService) validatePeerInvite(ctx context.Context, token, name, endpoint, trustDomain, peerType string) (string, error) {
	tokenHash := hashInviteToken(token)
	var inviteID, inviteName, inviteEndpoint, inviteTrustDomain, invitePeerType string
	var expiresAt time.Time
	err := fs.querier.QueryRow(ctx,
		`SELECT invite_id, name, endpoint, trust_domain, peer_type, expires_at
		 FROM federation_peer_invites
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&inviteID, &inviteName, &inviteEndpoint, &inviteTrustDomain, &invitePeerType, &expiresAt)
	if err != nil {
		return "", fmt.Errorf("invalid or expired federation invite")
	}
	if inviteEndpoint != endpoint {
		return "", fmt.Errorf("invite endpoint mismatch")
	}
	if inviteName != "" && inviteName != name {
		return "", fmt.Errorf("invite name mismatch")
	}
	if inviteTrustDomain != "" && trustDomain != "" && inviteTrustDomain != trustDomain {
		return "", fmt.Errorf("invite trust_domain mismatch")
	}
	if invitePeerType != "" && peerType != "" && invitePeerType != normalizePeerType(peerType) {
		return "", fmt.Errorf("invite peer_type mismatch")
	}
	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("federation invite expired")
	}
	return inviteID, nil
}

func (fs *FederationService) markPeerInviteUsed(ctx context.Context, inviteID string) error {
	_, err := fs.querier.Exec(ctx, `UPDATE federation_peer_invites SET used_at = NOW() WHERE invite_id = $1`, inviteID)
	if err != nil {
		return fmt.Errorf("mark federation invite used failed: %w", err)
	}
	return nil
}

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizePeerType(peerType string) string {
	validPeerTypes := map[string]bool{"self": true, "domestic": true, "remote": true, "community": true}
	if !validPeerTypes[peerType] {
		return "community"
	}
	return peerType
}

func validateFederationEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("endpoint must be an absolute http(s) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("endpoint scheme must be http or https")
	}
	return nil
}

func (fs *FederationService) GetPeer(ctx context.Context, gatewayID string) (*FederationPeer, error) {
	var peer FederationPeer
	var pubKeyBytes []byte
	var lastSync *time.Time
	var registeredAt time.Time

	err := fs.querier.QueryRow(ctx,
		`SELECT gateway_id, name, public_key, endpoint, trust_domain, peer_type, status, trust_score, agent_count, last_sync_at, registered_at
		 FROM federation_peers WHERE gateway_id = $1`,
		gatewayID,
	).Scan(&peer.GatewayID, &peer.Name, &pubKeyBytes, &peer.Endpoint, &peer.TrustDomain,
		&peer.PeerType, &peer.Status, &peer.TrustScore, &peer.AgentCount, &lastSync, &registeredAt)

	if err != nil {
		return nil, fmt.Errorf("peer not found: %w", err)
	}

	peer.PublicKey = base64.StdEncoding.EncodeToString(pubKeyBytes)
	peer.RegisteredAt = registeredAt.Format(time.RFC3339)
	if lastSync != nil {
		peer.LastSyncAt = lastSync.Format(time.RFC3339)
	}

	return &peer, nil
}

func (fs *FederationService) ListPeers(ctx context.Context, status, peerType string) ([]FederationPeer, error) {
	query := `SELECT gateway_id, name, public_key, endpoint, trust_domain, peer_type, status, trust_score, agent_count, last_sync_at, registered_at
		FROM federation_peers`
	args := []interface{}{}
	argIdx := 1
	conditions := []string{}

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if peerType != "" {
		conditions = append(conditions, fmt.Sprintf("peer_type = $%d", argIdx))
		args = append(args, peerType)
		argIdx++
	}
	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			query += " AND " + c
		}
	}
	query += " ORDER BY registered_at DESC"

	rows, err := fs.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list peers failed: %w", err)
	}
	defer rows.Close()

	var peers []FederationPeer
	for rows.Next() {
		var peer FederationPeer
		var pubKeyBytes []byte
		var lastSync *time.Time
		var registeredAt time.Time

		err := rows.Scan(&peer.GatewayID, &peer.Name, &pubKeyBytes, &peer.Endpoint, &peer.TrustDomain,
			&peer.PeerType, &peer.Status, &peer.TrustScore, &peer.AgentCount, &lastSync, &registeredAt)
		if err != nil {
			continue
		}

		peer.PublicKey = base64.StdEncoding.EncodeToString(pubKeyBytes)
		peer.RegisteredAt = registeredAt.Format(time.RFC3339)
		if lastSync != nil {
			peer.LastSyncAt = lastSync.Format(time.RFC3339)
		}

		peers = append(peers, peer)
	}

	if peers == nil {
		peers = []FederationPeer{}
	}

	return peers, nil
}

func (fs *FederationService) VerifyPassport(ctx context.Context, passport AgentPassport) error {
	if passport.AgentID == "" || passport.AgentPubKey == "" || passport.GatewayID == "" || passport.GatewaySig == "" || passport.IssuedAt == "" {
		return fmt.Errorf("passport requires agent_id, agent_public_key, gateway_id, gateway_signature, and issued_at")
	}
	issuedAt, err := time.Parse(time.RFC3339, passport.IssuedAt)
	if err != nil {
		return fmt.Errorf("invalid passport issued_at: %w", err)
	}
	if issuedAt.Before(time.Now().Add(-24*time.Hour)) || issuedAt.After(time.Now().Add(5*time.Minute)) {
		return fmt.Errorf("passport issued_at is stale or in the future")
	}

	peer, err := fs.GetPeer(ctx, passport.GatewayID)
	if err != nil {
		return fmt.Errorf("gateway not registered: %w", err)
	}
	if peer.Status != "active" {
		return fmt.Errorf("gateway %s is %s (not active)", passport.GatewayID, peer.Status)
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid gateway public_key: %w", err)
	}

	gatewayPubKey := ed25519.PublicKey(pubKeyBytes)

	sigBytes, err := base64.StdEncoding.DecodeString(passport.GatewaySig)
	if err != nil {
		return fmt.Errorf("invalid passport signature: %w", err)
	}

	verificationPayload := map[string]string{
		"agent_id":         passport.AgentID,
		"agent_public_key": passport.AgentPubKey,
		"gateway_id":       passport.GatewayID,
		"issued_at":        passport.IssuedAt,
	}
	payload, err := json.Marshal(verificationPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal verification payload: %w", err)
	}

	if !ed25519.Verify(gatewayPubKey, payload, sigBytes) {
		return fmt.Errorf("passport signature verification failed: signature does not match gateway public key")
	}

	return nil
}

func (fs *FederationService) SyncAgent(ctx context.Context, passport AgentPassport, name, owner string, trustScore float64, capabilities, allowedTools []string, description string, tags []string, scope string) (*FederatedAgent, error) {
	if err := fs.VerifyPassport(ctx, passport); err != nil {
		return nil, fmt.Errorf("passport verification failed: %w", err)
	}

	validScopes := map[string]bool{"domestic": true, "international": true}
	if !validScopes[scope] {
		scope = "international"
	}

	agentPubKeyBytes, err := base64.StdEncoding.DecodeString(passport.AgentPubKey)
	if err != nil {
		return nil, fmt.Errorf("invalid agent public_key: %w", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(passport.GatewaySig)
	if err != nil {
		return nil, fmt.Errorf("invalid passport signature: %w", err)
	}

	if capabilities == nil {
		capabilities = []string{}
	}
	if allowedTools == nil {
		allowedTools = []string{}
	}

	_, err = fs.querier.Exec(ctx,
		`INSERT INTO federated_agents (agent_id, gateway_id, name, owner, public_key, trust_score, capabilities, allowed_tools, passport_signature, passport_issued_at, scope, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'active', NOW(), NOW())
		 ON CONFLICT (agent_id) DO UPDATE SET
			name = $3, owner = $4, public_key = $5, trust_score = $6,
			capabilities = $7, allowed_tools = $8,
			passport_signature = $9, passport_issued_at = $10,
			scope = $11, status = 'active', updated_at = NOW()`,
		passport.AgentID, passport.GatewayID, name, owner, agentPubKeyBytes,
		trustScore, capabilities, allowedTools, sigBytes, passport.IssuedAt, scope,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sync federated agent: %w", err)
	}

	if description != "" || tags != nil {
		if tags == nil {
			tags = []string{}
		}
		servicesJSON, _ := json.Marshal([]string{})
		endpointsJSON, _ := json.Marshal(map[string]string{})
		_, err = fs.querier.Exec(ctx,
			`INSERT INTO federated_profiles (agent_id, description, services_offered, endpoints, tags, listed, updated_at)
			 VALUES ($1, $2, $3, $4, $5, true, NOW())
			 ON CONFLICT (agent_id) DO UPDATE SET
			 description = $2, tags = $5, updated_at = NOW()`,
			passport.AgentID, description, servicesJSON, endpointsJSON, tags,
		)
		if err != nil {
			slog.Error("failed to sync federated profile", "error", err)
		}
	}

	_, err = fs.querier.Exec(ctx,
		`INSERT INTO federated_heartbeats (agent_id, gateway_id, last_heartbeat, status, metadata, updated_at)
		 VALUES ($1, $2, NOW(), 'online', '{}', NOW())
		 ON CONFLICT (agent_id) DO UPDATE SET
		 last_heartbeat = NOW(), status = 'online', updated_at = NOW()`,
		passport.AgentID, passport.GatewayID,
	)
	if err != nil {
		slog.Error("failed to sync federated heartbeat", "error", err)
	}

	_, err = fs.querier.Exec(ctx,
		`UPDATE federation_peers SET agent_count = agent_count + 1, last_sync_at = NOW(), updated_at = NOW() WHERE gateway_id = $1`,
		passport.GatewayID,
	)
	if err != nil {
		slog.Error("failed to update peer agent_count", "error", err)
	}

	slog.Info("federated agent synced to airport", "agent_id", passport.AgentID, "gateway_id", passport.GatewayID, "name", name, "scope", scope)

	return &FederatedAgent{
		AgentID:           passport.AgentID,
		GatewayID:         passport.GatewayID,
		Name:              name,
		Owner:             owner,
		PublicKey:         passport.AgentPubKey,
		TrustScore:        trustScore,
		Capabilities:      capabilities,
		AllowedTools:      allowedTools,
		PassportSignature: passport.GatewaySig,
		PassportIssuedAt:  passport.IssuedAt,
		Scope:             scope,
		Status:            "active",
	}, nil
}

func (fs *FederationService) SyncMerchantTrust(ctx context.Context, in FederatedMerchantTrustSync) error {
	if in.MerchantID == "" || in.GatewayID == "" {
		return fmt.Errorf("merchant_id and gateway_id are required")
	}
	if in.TrustScore < 0 || in.TrustScore > 1 {
		return fmt.Errorf("trust_score must be in range 0..1")
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return fmt.Errorf("confidence must be in range 0..1")
	}
	peer, err := fs.GetPeer(ctx, in.GatewayID)
	if err != nil {
		return fmt.Errorf("unknown gateway: %w", err)
	}
	if peer.Status != "active" {
		return fmt.Errorf("gateway is not active")
	}
	if in.VerificationTier == "" {
		in.VerificationTier = "unverified"
	}
	if in.RiskFlags == nil {
		in.RiskFlags = []string{}
	}
	if in.OrderCount < 0 {
		in.OrderCount = 0
	}

	_, err = fs.querier.Exec(ctx, `
		INSERT INTO federated_merchant_trust
			(merchant_id, gateway_id, trust_score, confidence, verification_tier, risk_flags, hitl_only, suspended, order_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (merchant_id) DO UPDATE
		SET gateway_id = EXCLUDED.gateway_id,
			trust_score = EXCLUDED.trust_score,
			confidence = EXCLUDED.confidence,
			verification_tier = EXCLUDED.verification_tier,
			risk_flags = EXCLUDED.risk_flags,
			hitl_only = EXCLUDED.hitl_only,
			suspended = EXCLUDED.suspended,
			order_count = EXCLUDED.order_count,
			updated_at = NOW()
	`, in.MerchantID, in.GatewayID, in.TrustScore, in.Confidence, in.VerificationTier, in.RiskFlags, in.HITLOnly, in.Suspended, in.OrderCount)
	if err != nil {
		return fmt.Errorf("upsert federated merchant trust failed: %w", err)
	}

	if in.OrderID != "" && in.OutcomeStatus != "" && in.ReceiptSignature != "" {
		_, err = fs.querier.Exec(ctx, `
			INSERT INTO federated_merchant_trust_events
				(merchant_id, gateway_id, order_id, outcome_status, dispute_ref, receipt_signature, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7::jsonb, '{}'::jsonb))
			ON CONFLICT DO NOTHING
		`, in.MerchantID, in.GatewayID, in.OrderID, in.OutcomeStatus, in.DisputeRef, in.ReceiptSignature, in.Metadata)
		if err != nil {
			return fmt.Errorf("insert federated merchant trust event failed: %w", err)
		}
	}
	return nil
}

func (fs *FederationService) AuthorizeFederatedInvoke(ctx context.Context, req FederatedInvokeRequest) (*FederatedInvokeDecision, error) {
	if req.RequesterID == "" || req.ResponderID == "" || req.Action == "" {
		return nil, fmt.Errorf("requester_id, responder_id, and action are required")
	}

	var requesterTrust float64
	err := fs.querier.QueryRow(ctx,
		`SELECT trust_score FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.RequesterID,
	).Scan(&requesterTrust)
	if err != nil {
		return nil, fmt.Errorf("requester agent not found or inactive: %w", err)
	}

	var responderGatewayID, gatewayStatus string
	var responderTrust, gatewayTrust float64
	err = fs.querier.QueryRow(ctx,
		`SELECT fa.gateway_id, fa.trust_score, fg.status, fg.trust_score
		 FROM federated_agents fa
		 JOIN federation_peers fg ON fg.gateway_id = fa.gateway_id
		 WHERE fa.agent_id = $1 AND fa.status = 'active'`,
		req.ResponderID,
	).Scan(&responderGatewayID, &responderTrust, &gatewayStatus, &gatewayTrust)
	if err != nil {
		return nil, fmt.Errorf("responder federated agent not found or inactive: %w", err)
	}

	decision := &FederatedInvokeDecision{
		Allowed:            false,
		RequiresHITL:       false,
		Reason:             "cross-node invoke denied: trust or peer status insufficient",
		RiskLevel:          "high",
		RequesterTrust:     requesterTrust,
		ResponderTrust:     responderTrust,
		ResponderGatewayID: responderGatewayID,
		ExecutionEnabled:   false,
	}

	if gatewayStatus == "active" && gatewayTrust >= 0.5 && requesterTrust >= 0.5 && responderTrust >= 0.3 {
		decision.Allowed = true
		decision.Reason = "cross-node invoke authorized for policy-gated handoff"
		if requesterTrust < 0.7 {
			decision.RequiresHITL = true
			decision.Reason = "cross-node invoke authorized but requires HITL"
		}
	}

	outcome := "denied"
	if decision.Allowed {
		outcome = "success"
	}
	if decision.RequiresHITL {
		outcome = "hitl_required"
	}
	if err := fs.logFederatedConnection(ctx, req.RequesterID, req.ResponderID, responderGatewayID, req.Action, outcome, requesterTrust); err != nil {
		return nil, err
	}

	return decision, nil
}

func (fs *FederationService) logFederatedConnection(ctx context.Context, requesterID, responderID, responderGatewayID, action, outcome string, trustScore float64) error {
	_, err := fs.querier.Exec(ctx,
		`INSERT INTO federated_connections (requester_id, responder_id, responder_gateway_id, action, outcome, trust_score_at_time, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
		requesterID, responderID, responderGatewayID, action, outcome, trustScore,
	)
	if err != nil {
		return fmt.Errorf("log federated connection failed: %w", err)
	}
	return nil
}

func (fs *FederationService) FederatedHeartbeat(ctx context.Context, agentID, gatewayID, status string, metadata json.RawMessage) error {
	peer, err := fs.GetPeer(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("unknown gateway: %w", err)
	}
	if peer.Status != "active" {
		return fmt.Errorf("gateway is not active")
	}

	validStatuses := map[string]bool{"online": true, "offline": true, "busy": true, "idle": true}
	if !validStatuses[status] {
		status = "online"
	}
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	_, err = fs.querier.Exec(ctx,
		`INSERT INTO federated_heartbeats (agent_id, gateway_id, last_heartbeat, status, metadata, updated_at)
		 VALUES ($1, $2, NOW(), $3, $4, NOW())
		 ON CONFLICT (agent_id) DO UPDATE SET
		 last_heartbeat = NOW(), status = $3, metadata = $4, updated_at = NOW()`,
		agentID, gatewayID, status, metadata,
	)
	if err != nil {
		return fmt.Errorf("federated heartbeat failed: %w", err)
	}

	return nil
}

func (fs *FederationService) SearchFederatedAgents(ctx context.Context, status, tag, owner string, minTrust float64, scope string, limit, offset int) ([]FederatedAgent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	args := []interface{}{}
	argIdx := 1
	conditions := []string{"fa.status = 'active'", "COALESCE(fp.listed, true) = true", "fg.status = 'active'"}

	if minTrust > 0 {
		conditions = append(conditions, fmt.Sprintf("fa.trust_score >= $%d", argIdx))
		args = append(args, minTrust)
		argIdx++
	}

	if owner != "" {
		conditions = append(conditions, fmt.Sprintf("fa.owner = $%d", argIdx))
		args = append(args, owner)
		argIdx++
	}

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("fh.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	if tag != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(fp.tags)", argIdx))
		args = append(args, tag)
		argIdx++
	}

	if scope != "" {
		conditions = append(conditions, fmt.Sprintf("fa.scope = $%d", argIdx))
		args = append(args, scope)
		argIdx++
	}

	where := ""
	for i, c := range conditions {
		if i > 0 {
			where += " AND "
		}
		where += c
	}

	query := fmt.Sprintf(`SELECT fa.agent_id, fa.gateway_id, fa.name, fa.owner, fa.public_key,
		fa.trust_score, COALESCE(array_to_json(fa.capabilities)::text, '[]') as capabilities,
		COALESCE(array_to_json(fa.allowed_tools)::text, '[]') as allowed_tools,
		fa.passport_issued_at::text, fa.scope, fa.status,
		COALESCE(fp.description, '') as description,
		COALESCE(array_to_json(fp.tags)::text, '[]') as tags,
		COALESCE(fh.status, 'offline') as heartbeat_status,
		COALESCE(fh.last_heartbeat::text, '') as last_heartbeat,
		COALESCE(fmt.trust_score, 0.5) as merchant_trust_score,
		COALESCE(fmt.confidence, 0.1) as merchant_confidence,
		COALESCE(fmt.verification_tier, 'unverified') as merchant_verification_tier,
		COALESCE(array_to_json(fmt.risk_flags)::text, '[]') as merchant_risk_flags,
		COALESCE(fmt.hitl_only, false) as merchant_hitl_only,
		COALESCE(fmt.suspended, false) as merchant_suspended
		FROM federated_agents fa
		JOIN federation_peers fg ON fg.gateway_id = fa.gateway_id
		LEFT JOIN federated_profiles fp ON fp.agent_id = fa.agent_id
		LEFT JOIN federated_heartbeats fh ON fh.agent_id = fa.agent_id
		LEFT JOIN federated_merchant_trust fmt ON fmt.merchant_id = fa.agent_id
		WHERE %s
		ORDER BY fa.trust_score DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := fs.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("federated agent search failed: %w", err)
	}
	defer rows.Close()

	var agents []FederatedAgent
	for rows.Next() {
		var a FederatedAgent
		var pubKeyBytes []byte
		var capsStr, toolsStr, tagsStr, riskFlagsStr string

		err := rows.Scan(&a.AgentID, &a.GatewayID, &a.Name, &a.Owner, &pubKeyBytes,
			&a.TrustScore, &capsStr, &toolsStr,
			&a.PassportIssuedAt, &a.Scope, &a.Status,
			&a.Description, &tagsStr,
			&a.HeartbeatStatus, &a.LastHeartbeat,
			&a.MerchantTrustScore, &a.MerchantConfidence, &a.MerchantVerificationTier, &riskFlagsStr, &a.MerchantHITLOnly, &a.MerchantSuspended,
		)
		if err != nil {
			slog.Error("scan federated agent failed", "error", err)
			continue
		}

		a.PublicKey = base64.StdEncoding.EncodeToString(pubKeyBytes)
		json.Unmarshal([]byte(capsStr), &a.Capabilities)
		json.Unmarshal([]byte(toolsStr), &a.AllowedTools)
		json.Unmarshal([]byte(tagsStr), &a.Tags)
		json.Unmarshal([]byte(riskFlagsStr), &a.MerchantRiskFlags)

		if a.Capabilities == nil {
			a.Capabilities = []string{}
		}
		if a.AllowedTools == nil {
			a.AllowedTools = []string{}
		}
		if a.Tags == nil {
			a.Tags = []string{}
		}
		if a.MerchantRiskFlags == nil {
			a.MerchantRiskFlags = []string{}
		}

		agents = append(agents, a)
	}

	if agents == nil {
		agents = []FederatedAgent{}
	}

	return agents, nil
}

func (fs *FederationService) ListFederatedOnline(ctx context.Context, scope string) ([]FederatedAgent, error) {
	query := `SELECT fa.agent_id, fa.gateway_id, fa.name, fa.owner, fa.public_key,
		fa.trust_score, COALESCE(array_to_json(fa.capabilities)::text, '[]') as capabilities,
		COALESCE(array_to_json(fa.allowed_tools)::text, '[]') as allowed_tools,
		fa.passport_issued_at::text, fa.scope, fa.status,
		COALESCE(fp.description, '') as description,
		COALESCE(array_to_json(fp.tags)::text, '[]') as tags,
		fh.status as heartbeat_status,
		fh.last_heartbeat::text as last_heartbeat,
		COALESCE(fmt.trust_score, 0.5) as merchant_trust_score,
		COALESCE(fmt.confidence, 0.1) as merchant_confidence,
		COALESCE(fmt.verification_tier, 'unverified') as merchant_verification_tier,
		COALESCE(array_to_json(fmt.risk_flags)::text, '[]') as merchant_risk_flags,
		COALESCE(fmt.hitl_only, false) as merchant_hitl_only,
		COALESCE(fmt.suspended, false) as merchant_suspended
		FROM federated_agents fa
		JOIN federation_peers fg ON fg.gateway_id = fa.gateway_id
		JOIN federated_heartbeats fh ON fh.agent_id = fa.agent_id
		LEFT JOIN federated_profiles fp ON fp.agent_id = fa.agent_id
		LEFT JOIN federated_merchant_trust fmt ON fmt.merchant_id = fa.agent_id
		WHERE fh.status = 'online' AND fh.last_heartbeat > NOW() - INTERVAL '5 minutes'
		AND fa.status = 'active' AND COALESCE(fp.listed, true) = true AND fg.status = 'active'`
	args := []interface{}{}

	if scope != "" {
		query += ` AND fa.scope = $1`
		args = append(args, scope)
	}
	query += ` ORDER BY fa.trust_score DESC LIMIT 100`

	rows, err := fs.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list federated online failed: %w", err)
	}
	defer rows.Close()

	var agents []FederatedAgent
	for rows.Next() {
		var a FederatedAgent
		var pubKeyBytes []byte
		var capsStr, toolsStr, tagsStr, riskFlagsStr string

		err := rows.Scan(&a.AgentID, &a.GatewayID, &a.Name, &a.Owner, &pubKeyBytes,
			&a.TrustScore, &capsStr, &toolsStr,
			&a.PassportIssuedAt, &a.Scope, &a.Status,
			&a.Description, &tagsStr,
			&a.HeartbeatStatus, &a.LastHeartbeat,
			&a.MerchantTrustScore, &a.MerchantConfidence, &a.MerchantVerificationTier, &riskFlagsStr, &a.MerchantHITLOnly, &a.MerchantSuspended,
		)
		if err != nil {
			continue
		}

		a.PublicKey = base64.StdEncoding.EncodeToString(pubKeyBytes)
		json.Unmarshal([]byte(capsStr), &a.Capabilities)
		json.Unmarshal([]byte(toolsStr), &a.AllowedTools)
		json.Unmarshal([]byte(tagsStr), &a.Tags)
		json.Unmarshal([]byte(riskFlagsStr), &a.MerchantRiskFlags)

		if a.Capabilities == nil {
			a.Capabilities = []string{}
		}
		if a.AllowedTools == nil {
			a.AllowedTools = []string{}
		}
		if a.Tags == nil {
			a.Tags = []string{}
		}
		if a.MerchantRiskFlags == nil {
			a.MerchantRiskFlags = []string{}
		}

		agents = append(agents, a)
	}

	if agents == nil {
		agents = []FederatedAgent{}
	}

	return agents, nil
}

func (fs *FederationService) GetFederatedAgent(ctx context.Context, agentID string) (*FederatedAgent, error) {
	var a FederatedAgent
	var pubKeyBytes []byte
	var capsStr, toolsStr, tagsStr, riskFlagsStr string

	err := fs.querier.QueryRow(ctx,
		`SELECT fa.agent_id, fa.gateway_id, fa.name, fa.owner, fa.public_key,
		fa.trust_score, COALESCE(array_to_json(fa.capabilities)::text, '[]') as capabilities,
		COALESCE(array_to_json(fa.allowed_tools)::text, '[]') as allowed_tools,
		fa.passport_issued_at::text, fa.scope, fa.status,
		COALESCE(fp.description, '') as description,
		COALESCE(array_to_json(fp.tags)::text, '[]') as tags,
		COALESCE(fh.status, 'offline') as heartbeat_status,
		COALESCE(fh.last_heartbeat::text, '') as last_heartbeat,
		COALESCE(fmt.trust_score, 0.5) as merchant_trust_score,
		COALESCE(fmt.confidence, 0.1) as merchant_confidence,
		COALESCE(fmt.verification_tier, 'unverified') as merchant_verification_tier,
		COALESCE(array_to_json(fmt.risk_flags)::text, '[]') as merchant_risk_flags,
		COALESCE(fmt.hitl_only, false) as merchant_hitl_only,
		COALESCE(fmt.suspended, false) as merchant_suspended
		FROM federated_agents fa
		JOIN federation_peers fg ON fg.gateway_id = fa.gateway_id
		LEFT JOIN federated_profiles fp ON fp.agent_id = fa.agent_id
		LEFT JOIN federated_heartbeats fh ON fh.agent_id = fa.agent_id
		LEFT JOIN federated_merchant_trust fmt ON fmt.merchant_id = fa.agent_id
		WHERE fa.agent_id = $1 AND fg.status = 'active'`,
		agentID,
	).Scan(&a.AgentID, &a.GatewayID, &a.Name, &a.Owner, &pubKeyBytes,
		&a.TrustScore, &capsStr, &toolsStr,
		&a.PassportIssuedAt, &a.Scope, &a.Status,
		&a.Description, &tagsStr,
		&a.HeartbeatStatus, &a.LastHeartbeat,
		&a.MerchantTrustScore, &a.MerchantConfidence, &a.MerchantVerificationTier, &riskFlagsStr, &a.MerchantHITLOnly, &a.MerchantSuspended,
	)

	if err != nil {
		return nil, fmt.Errorf("federated agent not found: %w", err)
	}

	a.PublicKey = base64.StdEncoding.EncodeToString(pubKeyBytes)
	json.Unmarshal([]byte(capsStr), &a.Capabilities)
	json.Unmarshal([]byte(toolsStr), &a.AllowedTools)
	json.Unmarshal([]byte(tagsStr), &a.Tags)
	json.Unmarshal([]byte(riskFlagsStr), &a.MerchantRiskFlags)

	if a.Capabilities == nil {
		a.Capabilities = []string{}
	}
	if a.AllowedTools == nil {
		a.AllowedTools = []string{}
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}
	if a.MerchantRiskFlags == nil {
		a.MerchantRiskFlags = []string{}
	}

	return &a, nil
}

func (fs *FederationService) SuspendPeer(ctx context.Context, gatewayID string) error {
	_, err := fs.querier.Exec(ctx,
		`UPDATE federation_peers SET status = 'suspended', updated_at = NOW() WHERE gateway_id = $1`,
		gatewayID,
	)
	if err != nil {
		return fmt.Errorf("failed to suspend peer: %w", err)
	}

	_, err = fs.querier.Exec(ctx,
		`UPDATE federated_agents SET status = 'suspended', updated_at = NOW() WHERE gateway_id = $1`,
		gatewayID,
	)
	if err != nil {
		slog.Error("failed to suspend federated agents for peer", "gateway_id", gatewayID, "error", err)
	}

	slog.Info("federation peer suspended", "gateway_id", gatewayID)
	return nil
}

func (fs *FederationService) StartHeartbeatCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := fs.querier.Exec(ctx, `SELECT federated_mark_stale_offline()`)
				if err != nil {
					slog.Error("federated heartbeat cleanup failed", "error", err)
				} else {
					if result.RowsAffected > 0 {
						slog.Info("federated heartbeat cleanup", "marked_offline", result.RowsAffected)
					}
				}
			}
		}
	}()
}
