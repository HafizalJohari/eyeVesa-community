package grpcserver

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"log/slog"
	"time"

	pb "github.com/hafizaljohari/eyeVesa/proto/agentid"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GatewayServer struct {
	pb.UnimplementedGatewayServiceServer
	db               *database.DB
	auditLogger      *audit.AuditLogger
	gatewayPrivKey   ed25519.PrivateKey
	policyEngine     *policy.PolicyEngine
}

func NewGatewayServer(db *database.DB, auditLogger *audit.AuditLogger, privKey ed25519.PrivateKey, pe *policy.PolicyEngine) *GatewayServer {
	return &GatewayServer{
		db:             db,
		auditLogger:    auditLogger,
		gatewayPrivKey: privKey,
		policyEngine:   pe,
	}
}

func (s *GatewayServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	keypair, err := crypto.GenerateAgentKeypair()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate keypair: %v", err)
	}

	agentID := uuid.New()
	capabilities := req.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	allowedTools := req.AllowedTools
	if allowedTools == nil {
		allowedTools = []string{}
	}
	behavioralTags := req.BehavioralTags
	if behavioralTags == nil {
		behavioralTags = []string{}
	}
	delegationPolicy := req.DelegationPolicy
	if delegationPolicy == "" {
		delegationPolicy = "no_chain"
	}

	var createdAt time.Time
	err = s.db.Pool.QueryRow(ctx,
		`INSERT INTO agents (agent_id, name, owner, public_key, capabilities, allowed_tools, max_budget_usd, delegation_policy, behavioral_tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING created_at`,
		agentID, req.Name, req.Owner, keypair.PublicKey, capabilities, allowedTools,
		req.MaxBudgetUsd, delegationPolicy, behavioralTags,
	).Scan(&createdAt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	auditEntry := audit.AuditEntry{
		AgentID:     agentID.String(),
		Action:      "agent.register",
		Method:      "gRPC",
		Status:      "success",
		TrustBefore: 1.0,
		TrustAfter:  1.0,
	}
	if err := s.auditLogger.Log(ctx, auditEntry, s.gatewayPrivKey); err != nil {
		slog.Error("grpc audit log", "error", err)
	}

	return &pb.RegisterAgentResponse{
		AgentId:   agentID.String(),
		PublicKey: keypair.PublicKey,
		Status:    "active",
		TrustScore: 1.0,
		CreatedAt:  timestamppb.New(createdAt),
	}, nil
}

func (s *GatewayServer) RegisterResource(ctx context.Context, req *pb.RegisterResourceRequest) (*pb.RegisterResourceResponse, error) {
	resourceID := uuid.New()
	authMethod := req.AuthMethod
	if authMethod == "" {
		authMethod = "mTLS+SVID"
	}
	riskLevel := req.RiskLevel
	if riskLevel == "" {
		riskLevel = "medium"
	}
	dataSensitivity := req.DataSensitivity
	if dataSensitivity == "" {
		dataSensitivity = "internal"
	}
	rateLimit := req.RateLimitPerAgent
	if rateLimit == 0 {
		rateLimit = 100
	}
	capabilitiesJSON := json.RawMessage(req.CapabilitiesJson)
	if len(capabilitiesJSON) == 0 {
		capabilitiesJSON = json.RawMessage(`{}`)
	}

	var createdAt time.Time
	err := s.db.Pool.QueryRow(ctx,
		`INSERT INTO resources (resource_id, name, resource_type, endpoint, auth_method, capabilities, risk_level, data_sensitivity, rate_limit_per_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING created_at`,
		resourceID, req.Name, req.Type, req.Endpoint, authMethod,
		capabilitiesJSON, riskLevel, dataSensitivity, rateLimit,
	).Scan(&createdAt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	return &pb.RegisterResourceResponse{
		ResourceId: resourceID.String(),
		CreatedAt:  timestamppb.New(createdAt),
	}, nil
}

func (s *GatewayServer) Authorize(ctx context.Context, req *pb.AuthorizeRequest) (*pb.AuthorizeResponse, error) {
	if req.AgentId == "" || req.Action == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id and action are required")
	}

	var owner string
	var trustScore float64
	var capabilities, allowedTools []string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT owner, trust_score, capabilities, allowed_tools FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.AgentId,
	).Scan(&owner, &trustScore, &capabilities, &allowedTools)

	if err != nil {
		return &pb.AuthorizeResponse{
			Allowed: false,
			Reason:  "agent not found or inactive",
		}, nil
	}

	policyInput := policy.PolicyInput{}
	policyInput.Agent.ID = req.AgentId
	policyInput.Agent.Owner = owner
	policyInput.Agent.TrustScore = trustScore
	policyInput.Agent.AllowedTools = allowedTools
	policyInput.Action.Tool = req.Action
	policyInput.Action.ResourceID = req.ResourceId

	var params map[string]interface{}
	if req.ParamsJson != "" {
		if err := json.Unmarshal([]byte(req.ParamsJson), &params); err != nil {
			slog.Warn("grpc authorize params unmarshal", "error", err)
		}
		policyInput.Action.Params = params
	}

	decision := s.policyEngine.Evaluate(ctx, policyInput)

	newTrustScore := trustScore + decision.TrustDelta
	if newTrustScore < 0 {
		newTrustScore = 0
	}
	if newTrustScore > 1 {
		newTrustScore = 1
	}

	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE agents SET trust_score = $1, updated_at = NOW() WHERE agent_id = $2`,
		newTrustScore, req.AgentId,
	); err != nil {
		slog.Error("grpc trust update", "error", err)
	}

	if _, err := s.db.Pool.Exec(ctx,
		`INSERT INTO trust_events (agent_id, event_type, trust_delta, trust_score_after, reason) VALUES ($1, $2, $3, $4, $5)`,
		req.AgentId, "authorize", decision.TrustDelta, newTrustScore, decision.Reason,
	); err != nil {
		slog.Error("grpc trust event insert", "error", err)
	}

	auditEntry := audit.AuditEntry{
		AgentID:     req.AgentId,
		ResourceID:  req.ResourceId,
		Action:      req.Action,
		Method:      "gRPC",
		Status:      map[bool]string{true: "allowed", false: "denied"}[decision.Allowed],
		TrustBefore: trustScore,
		TrustAfter:  newTrustScore,
	}
	if err := s.auditLogger.Log(ctx, auditEntry, s.gatewayPrivKey); err != nil {
		slog.Error("grpc audit log", "error", err)
	}

	return &pb.AuthorizeResponse{
		Allowed:      decision.Allowed,
		RequiresHitl: decision.RequiresHITL,
		Reason:       decision.Reason,
		TrustDelta:   decision.TrustDelta,
	}, nil
}

func (s *GatewayServer) VerifySignature(ctx context.Context, req *pb.VerifySignatureRequest) (*pb.VerifySignatureResponse, error) {
	var pubKeyBytes []byte
	err := s.db.Pool.QueryRow(ctx,
		`SELECT public_key FROM agents WHERE agent_id = $1`,
		req.AgentId,
	).Scan(&pubKeyBytes)

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
	}

	valid := crypto.VerifySignature(pubKeyBytes, req.Message, req.Signature)

	return &pb.VerifySignatureResponse{
		Valid:   valid,
		AgentId: req.AgentId,
	}, nil
}

func (s *GatewayServer) GetAgent(ctx context.Context, req *pb.GetAgentRequest) (*pb.GetAgentResponse, error) {
	var name, owner, agentStatus string
	var trustScore float64
	var capabilities, allowedTools []string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT name, owner, trust_score, status, capabilities, allowed_tools FROM agents WHERE agent_id = $1`,
		req.AgentId,
	).Scan(&name, &owner, &trustScore, &agentStatus, &capabilities, &allowedTools)

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent not found: %v", err)
	}

	return &pb.GetAgentResponse{
		AgentId:      req.AgentId,
		Name:         name,
		Owner:        owner,
		TrustScore:   trustScore,
		Status:       agentStatus,
		Capabilities: capabilities,
		AllowedTools: allowedTools,
	}, nil
}

func (s *GatewayServer) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	rows, err := s.db.Pool.Query(ctx,
		`SELECT agent_id, name, owner, trust_score, status, capabilities, allowed_tools FROM agents ORDER BY created_at DESC`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}
	defer rows.Close()

	var agents []*pb.GetAgentResponse
	for rows.Next() {
		var id, name, owner, agentStatus string
		var trustScore float64
		var capabilities, allowedTools []string
		if err := rows.Scan(&id, &name, &owner, &trustScore, &agentStatus, &capabilities, &allowedTools); err != nil {
			continue
		}
		agents = append(agents, &pb.GetAgentResponse{
			AgentId:      id,
			Name:         name,
			Owner:        owner,
			TrustScore:   trustScore,
			Status:       agentStatus,
			Capabilities: capabilities,
			AllowedTools: allowedTools,
		})
	}

	return &pb.ListAgentsResponse{Agents: agents}, nil
}

func (s *GatewayServer) Audit(ctx context.Context, req *pb.AuditRequest) (*pb.AuditResponse, error) {
	limit := int32(50)
	if req.Limit > 0 {
		limit = req.Limit
	}

	rows, err := s.db.Pool.Query(ctx,
		`SELECT log_id, agent_id, COALESCE(resource_id::text, ''), action, result_status, trust_score_before, trust_score_after, created_at FROM audit_logs WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2`,
		req.AgentId, limit,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}
	defer rows.Close()

	var entries []*pb.AuditEntry
	for rows.Next() {
		var logID, agentID, resourceID, action, resultStatus string
		var trustBefore, trustAfter float64
		var createdAt time.Time
		if err := rows.Scan(&logID, &agentID, &resourceID, &action, &resultStatus, &trustBefore, &trustAfter, &createdAt); err != nil {
			continue
		}
		entries = append(entries, &pb.AuditEntry{
			LogId:           logID,
			AgentId:         agentID,
			ResourceId:      resourceID,
			Action:          action,
			ResultStatus:    resultStatus,
			TrustScoreBefore: trustBefore,
			TrustScoreAfter: trustAfter,
			CreatedAt:       timestamppb.New(createdAt),
		})
	}

	return &pb.AuditResponse{Entries: entries}, nil
}