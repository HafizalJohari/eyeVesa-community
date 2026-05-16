package hitl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApprovalRequest struct {
	AgentID    string                 `json:"agent_id"`
	ResourceID string                 `json:"resource_id"`
	Action     string                 `json:"action"`
	Params     map[string]interface{} `json:"params"`
	Reason     string                 `json:"reason"`
	RiskLevel  string                 `json:"risk_level"`
}

type ApprovalResponse struct {
	ApprovalID string `json:"approval_id"`
	AgentID    string `json:"agent_id"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	ExpiresAt  string `json:"expires_at"`
}

type ApprovalDecision struct {
	ApprovalID     string `json:"approval_id"`
	Approved       bool   `json:"approved"`
	ApproverMethod string `json:"approver_method"`
}

type HITLService struct {
	db *pgxpool.Pool
}

func NewHITLService(db *pgxpool.Pool) *HITLService {
	return &HITLService{db: db}
}

func (s *HITLService) RequestApproval(ctx context.Context, req ApprovalRequest) (*ApprovalResponse, error) {
	approvalID := uuid.New()
	expiresAt := time.Now().Add(5 * time.Minute)

	reason := req.Reason
	if reason == "" {
		reason = fmt.Sprintf("Agent %s requests action '%s' on resource %s",
			req.AgentID, req.Action, req.ResourceID)
	}

	riskLevel := req.RiskLevel
	if riskLevel == "" {
		riskLevel = "medium"
	}

	paramsJSON := "{}"
	if req.Params != nil {
		if b, err := jsonParams(req.Params); err == nil {
			paramsJSON = b
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO hitl_approvals (approval_id, agent_id, resource_id, action, params, status, expires_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, 'pending', $6)`,
		approvalID, req.AgentID, nilIfEmpty(req.ResourceID), req.Action, paramsJSON, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create approval request: %w", err)
	}

	return &ApprovalResponse{
		ApprovalID: approvalID.String(),
		AgentID:    req.AgentID,
		Action:     req.Action,
		Status:     "pending",
		ExpiresAt:  expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *HITLService) Approve(ctx context.Context, decision ApprovalDecision) error {
	approvedStatus := "approved"
	if !decision.Approved {
		approvedStatus = "rejected"
	}

	result, err := s.db.Exec(ctx,
		`UPDATE hitl_approvals SET status = $1, approval_method = $2, approved_at = NOW()
		 WHERE approval_id = $3 AND status = 'pending'`,
		approvedStatus, decision.ApproverMethod, decision.ApprovalID,
	)
	if err != nil {
		return fmt.Errorf("failed to update approval: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval not found or already processed")
	}

	return nil
}

func (s *HITLService) GetStatus(ctx context.Context, approvalID string) (string, error) {
	var status string
	err := s.db.QueryRow(ctx,
		`SELECT status FROM hitl_approvals WHERE approval_id = $1`,
		approvalID,
	).Scan(&status)

	if err != nil {
		return "", fmt.Errorf("approval not found: %w", err)
	}

	return status, nil
}

func (s *HITLService) ListPending(ctx context.Context, agentID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx,
		`SELECT approval_id, agent_id, action, status, expires_at
		 FROM hitl_approvals WHERE status = 'pending' AND ($1 = '' OR agent_id = $1)
		 ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, agID, action, status string
		var expiresAt time.Time
		if err := rows.Scan(&id, &agID, &action, &status, &expiresAt); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"approval_id": id,
			"agent_id":    agID,
			"action":      action,
			"status":      status,
			"expires_at":  expiresAt.Format(time.RFC3339),
		})
	}
	return results, nil
}

func (s *HITLService) ExpireOld(ctx context.Context) (int64, error) {
	result, err := s.db.Exec(ctx,
		`UPDATE hitl_approvals SET status = 'expired'
		 WHERE status = 'pending' AND expires_at < NOW()`,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func jsonParams(params map[string]interface{}) (string, error) {
	b, err := json.Marshal(params)
	return string(b), err
}