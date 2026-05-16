package hitl

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EscalationLevel int

const (
	LevelAutoAllow EscalationLevel = 0
	LevelHITL      EscalationLevel = 1
	LevelEscalated EscalationLevel = 2
	LevelAutoDeny  EscalationLevel = -1
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type NotificationChannel string

const (
	ChannelWebhook   NotificationChannel = "webhook"
	ChannelSlack     NotificationChannel = "slack"
	ChannelEmail     NotificationChannel = "email"
	ChannelPagerduty NotificationChannel = "pagerduty"
	ChannelPush      NotificationChannel = "push"
)

type ApprovalChainEntry struct {
	ChainID        string     `json:"chain_id"`
	ApprovalID     string     `json:"approval_id"`
	ApproverID     string     `json:"approver_id"`
	ApprovalLevel int        `json:"approval_level"`
	Decision       string     `json:"decision"`
	DecisionReason string     `json:"decision_reason,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type EscalationConfig struct {
	ConfigID       string             `json:"config_id"`
	TenantID       string             `json:"tenant_id,omitempty"`
	Level          int                `json:"level"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	NotifyChannel  NotificationChannel `json:"notify_channel"`
	NotifyTarget   string             `json:"notify_target"`
	CreatedAt      time.Time          `json:"created_at"`
}

type NotificationEntry struct {
	NotificationID   string     `json:"notification_id"`
	ApprovalID       string     `json:"approval_id"`
	Channel          string     `json:"channel"`
	RecipientID      string     `json:"recipient_id"`
	Message          string     `json:"message,omitempty"`
	SentAt           time.Time  `json:"sent_at"`
	AcknowledgedAt   *time.Time `json:"acknowledged_at,omitempty"`
	EscalationLevel  int        `json:"escalation_level"`
}

func DetermineEscalationLevel(agentTrustScore float64, tool string, params map[string]interface{}, resourceRiskLevel string) (EscalationLevel, RiskLevel) {
	if agentTrustScore < 0.1 {
		return LevelAutoDeny, RiskCritical
	}

	if tool == "bank_transfer" {
		amount, _ := params["amount"].(float64)
		if amount > 5000 {
			return LevelAutoDeny, RiskCritical
		}
		if amount > 1000 {
			return LevelEscalated, RiskCritical
		}
		if amount > 100 {
			return LevelHITL, RiskHigh
		}
	}

	if tool == "database_schema_change" {
		return LevelEscalated, RiskCritical
	}

	if agentTrustScore < 0.5 && resourceRiskLevel == "high" {
		return LevelEscalated, RiskHigh
	}

	if resourceRiskLevel == "restricted" && agentTrustScore < 0.8 {
		return LevelHITL, RiskHigh
	}

	if tool == "k8s_deploy" {
		ns, _ := params["namespace"].(string)
		if ns == "production" {
			return LevelHITL, RiskHigh
		}
	}

	if agentTrustScore >= 0.8 && (resourceRiskLevel == "low" || resourceRiskLevel == "") {
		return LevelAutoAllow, RiskLow
	}

	if agentTrustScore >= 0.5 {
		return LevelAutoAllow, RiskLow
	}

	return LevelHITL, RiskMedium
}

func RequiredApprovals(escalationLevel EscalationLevel) int {
	switch escalationLevel {
	case LevelEscalated:
		return 2
	case LevelHITL:
		return 1
	default:
		return 0
	}
}

func TrustDeltaForDecision(status string) float64 {
	switch status {
	case "approved":
		return 0.01
	case "rejected":
		return -0.02
	case "expired":
		return -0.01
	default:
		return 0
	}
}

type EscalationService struct {
	db             *pgxpool.Pool
	notifyChans    map[NotificationChannel]Notifier
}

type Notifier interface {
	Send(ctx context.Context, target string, message string) error
}

func NewEscalationService(db *pgxpool.Pool) *EscalationService {
	svc := &EscalationService{
		db:          db,
		notifyChans: make(map[NotificationChannel]Notifier),
	}
	return svc
}

func (s *EscalationService) RegisterNotifier(channel NotificationChannel, notifier Notifier) {
	s.notifyChans[channel] = notifier
}

func (s *EscalationService) RequestEscalatedApproval(ctx context.Context, req ApprovalRequest) (*ApprovalResponse, error) {
	approvalID := uuid.New()
	escalationLevel := LevelHITL
	riskLevel := RiskMedium
	requiredApprovals := 1

	var trustScore float64
	err := s.db.QueryRow(ctx,
		`SELECT trust_score FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.AgentID,
	).Scan(&trustScore)

	if err == nil {
		var resourceRisk string
		_ = s.db.QueryRow(ctx,
			`SELECT risk_level FROM resources WHERE resource_id = $1`,
			req.ResourceID,
		).Scan(&resourceRisk)

		escalationLevel, riskLevel = DetermineEscalationLevel(trustScore, req.Action, req.Params, resourceRisk)
		requiredApprovals = RequiredApprovals(escalationLevel)

		if escalationLevel == LevelAutoDeny {
			return nil, fmt.Errorf("auto-deny: action '%s' blocked by policy (trust: %.2f, risk: %s)", req.Action, trustScore, riskLevel)
		}
	}

	if escalationLevel == LevelAutoAllow {
		resp := &ApprovalResponse{
			ApprovalID: approvalID.String(),
			AgentID:    req.AgentID,
			Action:     req.Action,
			Status:     "auto_allowed",
			ExpiresAt:  time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		}
		return resp, nil
	}

	expiresAt := time.Now().Add(30 * time.Minute)

	reason := req.Reason
	if reason == "" {
		reason = fmt.Sprintf("Agent %s requests '%s' (risk: %s, level: %d)",
			req.AgentID, req.Action, riskLevel, escalationLevel)
	}

	paramsJSON := "{}"
	if req.Params != nil {
		if b, err := jsonParams(req.Params); err == nil {
			paramsJSON = b
		}
	}

	resourceID := nilIfEmpty(req.ResourceID)
	_, err = s.db.Exec(ctx,
		`INSERT INTO hitl_approvals (approval_id, agent_id, resource_id, action, params, status, risk_level, required_approvals, current_approvals, escalation_level, expires_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, 'pending', $6, $7, 0, $8, $9)`,
		approvalID, req.AgentID, resourceID, req.Action, paramsJSON, string(riskLevel), requiredApprovals, int(escalationLevel), expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create escalated approval: %w", err)
	}

	s.enqueueNotification(ctx, approvalID.String(), 0, reason)

	s.escalateNotify(ctx, approvalID.String(), int(escalationLevel), requiredApprovals)

	return &ApprovalResponse{
		ApprovalID: approvalID.String(),
		AgentID:    req.AgentID,
		Action:     req.Action,
		Status:     "pending",
		ExpiresAt:  expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *EscalationService) ProcessChainDecision(ctx context.Context, approvalID string, approverID string, approved bool, reason string) (*ApprovalChainEntry, error) {
	var currentApprovals int
	var requiredApprovals int
	var currentStatus string
	var escalationLevel int

	err := s.db.QueryRow(ctx,
		`SELECT current_approvals, required_approvals, status, escalation_level FROM hitl_approvals WHERE approval_id = $1`,
		approvalID,
	).Scan(&currentApprovals, &requiredApprovals, &currentStatus, &escalationLevel)

	if err != nil {
		return nil, fmt.Errorf("approval not found: %w", err)
	}

	if currentStatus != "pending" {
		return nil, fmt.Errorf("approval already %s", currentStatus)
	}

	decision := "approved"
	if !approved {
		decision = "rejected"
	}

	approvalLevel := currentApprovals + 1

	chainID := uuid.New()
	_, err = s.db.Exec(ctx,
		`INSERT INTO hitl_approval_chain (chain_id, approval_id, approver_id, approval_level, decision, decision_reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		chainID, approvalID, approverID, approvalLevel, decision, reason,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record chain decision: %w", err)
	}

	newStatus := "pending"
	newApprovals := currentApprovals

	if !approved {
		newStatus = "rejected"
		newApprovals = currentApprovals
	} else {
		newApprovals = currentApprovals + 1
		if newApprovals >= requiredApprovals {
			newStatus = "approved"
		}
	}

	_, err = s.db.Exec(ctx,
		`UPDATE hitl_approvals SET current_approvals = $1, status = $2, approval_method = $3, approved_at = CASE WHEN $2 != 'pending' THEN NOW() ELSE NULL END
		 WHERE approval_id = $4`,
		newApprovals, newStatus, fmt.Sprintf("chain_level_%d", approvalLevel), approvalID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update approval: %w", err)
	}

	if newStatus != "pending" {
		delta := TrustDeltaForDecision(newStatus)
		s.updateTrustForApproval(ctx, approvalID, delta, newStatus)
	}

	return &ApprovalChainEntry{
		ChainID:        chainID.String(),
		ApprovalID:     approvalID,
		ApproverID:     approverID,
		ApprovalLevel:  approvalLevel,
		Decision:       decision,
		DecisionReason: reason,
		CreatedAt:      time.Now(),
	}, nil
}

func (s *EscalationService) GetApprovalChain(ctx context.Context, approvalID string) ([]ApprovalChainEntry, error) {
	rows, err := s.db.Query(ctx,
		`SELECT chain_id, approval_id, approver_id, approval_level, decision, decision_reason, decided_at, created_at
		 FROM hitl_approval_chain WHERE approval_id = $1 ORDER BY approval_level`,
		approvalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ApprovalChainEntry
	for rows.Next() {
		var e ApprovalChainEntry
		var decidedAt *time.Time
		if err := rows.Scan(&e.ChainID, &e.ApprovalID, &e.ApproverID, &e.ApprovalLevel, &e.Decision, &e.DecisionReason, &decidedAt, &e.CreatedAt); err != nil {
			continue
		}
		e.DecidedAt = decidedAt
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *EscalationService) EnforceRateLimit(ctx context.Context, agentID string, resourceID string, limit int) (bool, int, error) {
	windowStart := time.Now().Add(-time.Minute)
	windowEnd := time.Now()

	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(request_count), 0) FROM rate_limit_counters
		 WHERE agent_id = $1 AND ($2::uuid IS NULL OR resource_id = $2) AND window_start >= $3 AND window_end <= $4`,
		agentID, nilIfEmpty(resourceID), windowStart, windowEnd,
	).Scan(&count)
	if err != nil {
		return true, 0, nil
	}

	if count >= limit {
		return false, count, nil
	}

	return true, count, nil
}

func (s *EscalationService) RecordSpend(ctx context.Context, agentID string, resourceID string, action string, estimatedCost float64, actualCost float64) error {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)

	_, err := s.db.Exec(ctx,
		`INSERT INTO agent_spend (agent_id, resource_id, action, estimated_cost, actual_cost, period, period_start, period_end)
		 VALUES ($1, $2, $3, $4, $5, 'monthly', $6, $7)`,
		agentID, nilIfEmpty(resourceID), action, estimatedCost, actualCost, periodStart, periodEnd,
	)
	return err
}

func (s *EscalationService) CheckBudget(ctx context.Context, agentID string, estimatedCost float64) (bool, float64, error) {
	var maxBudget float64
	err := s.db.QueryRow(ctx,
		`SELECT max_budget_usd FROM agents WHERE agent_id = $1`, agentID,
	).Scan(&maxBudget)
	if err != nil {
		return false, 0, err
	}

	if maxBudget <= 0 {
		return true, maxBudget, nil
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)

	var totalSpend float64
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(estimated_cost), 0) FROM agent_spend
		 WHERE agent_id = $1 AND period_start >= $2 AND period_end <= $3`,
		agentID, periodStart, periodEnd,
	).Scan(&totalSpend)

	remaining := maxBudget - totalSpend
	return remaining >= estimatedCost, remaining, nil
}

func (s *EscalationService) RunEscalationTicker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processEscalationTimers(ctx)
		}
	}
}

func (s *EscalationService) processEscalationTimers(ctx context.Context) {
	s.expireOldApprovals(ctx)
	s.escalateStaleApprovals(ctx)
}

func (s *EscalationService) expireOldApprovals(ctx context.Context) {
	s.db.Exec(ctx,
		`UPDATE hitl_approvals SET status = 'expired'
		 WHERE status = 'pending' AND expires_at < NOW()`,
	)

	s.db.Exec(ctx,
		`UPDATE agents a SET trust_score = GREATEST(0, trust_score - 0.01)
		 WHERE agent_id IN (SELECT agent_id FROM hitl_approvals WHERE status = 'expired' AND expires_at < NOW() - INTERVAL '1 minute')`,
	)
}

func (s *EscalationService) escalateStaleApprovals(ctx context.Context) {
	rows, err := s.db.Query(ctx,
		`SELECT approval_id, escalation_level, required_approvals, current_approvals FROM hitl_approvals
		 WHERE status = 'pending' AND created_at < NOW() - INTERVAL '5 minutes'`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var approvalID string
		var level, req, cur int
		if err := rows.Scan(&approvalID, &level, &req, &cur); err != nil {
			continue
		}

		s.db.Exec(ctx,
			`UPDATE hitl_approvals SET escalation_level = escalation_level + 1 WHERE approval_id = $1`,
			approvalID,
		)

		s.enqueueNotification(ctx, approvalID, level+1,
			fmt.Sprintf("ESCALATION: Approval %s has been pending for 5+ minutes", approvalID))
	}
}

func (s *EscalationService) enqueueNotification(ctx context.Context, approvalID string, escalationLevel int, message string) {
	notifierID := uuid.New()

	channel := ChannelWebhook
	target := "default"

	configRows, err := s.db.Query(ctx,
		`SELECT notify_channel, notify_target FROM hitl_escalation_config WHERE level = $1 ORDER BY config_id LIMIT 1`,
		escalationLevel,
	)
	if err == nil {
		defer configRows.Close()
		if configRows.Next() {
			var ch, tgt string
			if configRows.Scan(&ch, &tgt) == nil {
				channel = NotificationChannel(ch)
				target = tgt
			}
		}
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO hitl_notifications (notification_id, approval_id, channel, recipient_id, message, escalation_level)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		notifierID, approvalID, string(channel), "system", message, escalationLevel,
	)
	if err != nil {
		return
	}

	if notifier, ok := s.notifyChans[channel]; ok {
		notifier.Send(ctx, target, message)
	}

	s.pushToApprovers(ctx, approvalID, message)
}

func (s *EscalationService) pushToApprovers(ctx context.Context, approvalID string, message string) {
	pushNotifier, ok := s.notifyChans[ChannelPush]
	if !ok {
		return
	}

	rows, err := s.db.Query(ctx,
		`SELECT pt.device_token, pt.platform FROM push_tokens pt
		 JOIN approvers a ON pt.approver_id = a.approver_id
		 WHERE pt.is_active = TRUE AND a.is_active = TRUE`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var deviceToken, platform string
		if err := rows.Scan(&deviceToken, &platform); err != nil {
			continue
		}

		prefix := "fcm:"
		if platform == "ios" || platform == "apns" {
			prefix = "apns:"
		}

		pushNotifier.Send(ctx, prefix+deviceToken, message)
	}
}

func (s *EscalationService) updateTrustForApproval(ctx context.Context, approvalID string, delta float64, reason string) {
	var agentID string
	err := s.db.QueryRow(ctx,
		`SELECT agent_id FROM hitl_approvals WHERE approval_id = $1`, approvalID,
	).Scan(&agentID)
	if err != nil {
		return
	}

	s.db.Exec(ctx,
		`UPDATE agents SET trust_score = GREATEST(0, LEAST(1, trust_score + $1)), updated_at = NOW() WHERE agent_id = $2`,
		delta, agentID,
	)

	s.db.Exec(ctx,
		`INSERT INTO trust_events (agent_id, event_type, trust_delta, trust_score_after, reason)
		 SELECT agent_id, 'hitl_approval', $1, trust_score, $2 FROM agents WHERE agent_id = $3`,
		delta, reason, agentID,
	)
}

func (s *EscalationService) GetNotifications(ctx context.Context, approvalID string) ([]NotificationEntry, error) {
	rows, err := s.db.Query(ctx,
		`SELECT notification_id, approval_id, channel, recipient_id, message, sent_at, acknowledged_at, escalation_level
		 FROM hitl_notifications WHERE approval_id = $1 ORDER BY sent_at`,
		approvalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []NotificationEntry
	for rows.Next() {
		var e NotificationEntry
		if err := rows.Scan(&e.NotificationID, &e.ApprovalID, &e.Channel, &e.RecipientID, &e.Message, &e.SentAt, &e.AcknowledgedAt, &e.EscalationLevel); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *EscalationService) escalateNotify(ctx context.Context, approvalID string, escalationLevel int, requiredApprovals int) {
	if escalationLevel >= 2 {
		s.enqueueNotification(ctx, approvalID, 2,
			fmt.Sprintf("ESCALATION: Approval %s requires %d separate approvals", approvalID, requiredApprovals))
	} else if escalationLevel == 1 {
		s.enqueueNotification(ctx, approvalID, 1,
			fmt.Sprintf("HITL: Approval %s requires 1 approval", approvalID))
	}
}