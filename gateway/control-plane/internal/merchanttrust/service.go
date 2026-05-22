package merchanttrust

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

const (
	roleMerchant = "merchant"
)

type OutcomeEvent struct {
	MerchantID       string                 `json:"merchant_id"`
	BuyerAgentID     string                 `json:"buyer_agent_id,omitempty"`
	OutcomeType      string                 `json:"outcome_type"`
	OrderID          string                 `json:"order_id,omitempty"`
	DisputeRef       string                 `json:"dispute_ref,omitempty"`
	ReceiptSignature string                 `json:"receipt_signature,omitempty"`
	EventTS          *time.Time             `json:"event_ts,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type FeedbackEvent struct {
	MerchantID        string                 `json:"merchant_id"`
	BuyerAgentID      string                 `json:"buyer_agent_id,omitempty"`
	Stars             int                    `json:"stars"`
	SentimentScore    float64                `json:"sentiment_score"`
	ComplaintSeverity int                    `json:"complaint_severity"`
	OrderID           string                 `json:"order_id,omitempty"`
	EventTS           *time.Time             `json:"event_ts,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type State struct {
	MerchantID           string    `json:"merchant_id"`
	TrustScore           float64   `json:"trust_score"`
	Confidence           float64   `json:"confidence"`
	VolumeBucket         string    `json:"volume_bucket"`
	RiskFlags            []string  `json:"risk_flags"`
	TotalObjectiveEvents int       `json:"total_objective_events"`
	TotalFeedbackEvents  int       `json:"total_feedback_events"`
	Suspended            bool      `json:"suspended"`
	HITLOnly             bool      `json:"hitl_only"`
	LastUpdatedAt        time.Time `json:"last_updated_at"`
}

type Service struct {
	q database.Querier
}

func NewService(q database.Querier) *Service {
	return &Service{q: q}
}

func (s *Service) EnsureMerchantRole(ctx context.Context, merchantID string) error {
	_, err := s.q.Exec(ctx, `
		UPDATE agents
		SET roles = CASE WHEN NOT ($2 = ANY(roles)) THEN array_append(roles, $2) ELSE roles END,
			updated_at = NOW()
		WHERE agent_id = $1
	`, merchantID, roleMerchant)
	return err
}

func (s *Service) UpsertMerchantProfile(ctx context.Context, merchantID, businessType, fulfillmentModel, supportSLA, verificationTier string, categories, regions []string) error {
	if businessType == "" {
		businessType = "digital_goods"
	}
	if fulfillmentModel == "" {
		fulfillmentModel = "api"
	}
	if supportSLA == "" {
		supportSLA = "best_effort"
	}
	if verificationTier == "" {
		verificationTier = "unverified"
	}
	if categories == nil {
		categories = []string{}
	}
	if regions == nil {
		regions = []string{}
	}

	_, err := s.q.Exec(ctx, `
		INSERT INTO merchant_profiles (merchant_id, business_type, categories, fulfillment_model, regions, support_sla, verification_tier, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (merchant_id) DO UPDATE
		SET business_type = $2, categories = $3, fulfillment_model = $4, regions = $5, support_sla = $6, verification_tier = $7, updated_at = NOW()
	`, merchantID, businessType, categories, fulfillmentModel, regions, supportSLA, verificationTier)
	if err != nil {
		return err
	}

	_, err = s.q.Exec(ctx, `
		INSERT INTO merchant_trust_state (merchant_id)
		VALUES ($1)
		ON CONFLICT (merchant_id) DO NOTHING
	`, merchantID)
	return err
}

func (s *Service) IngestOutcome(ctx context.Context, in OutcomeEvent) (*State, error) {
	if in.MerchantID == "" || in.OutcomeType == "" {
		return nil, fmt.Errorf("merchant_id and outcome_type are required")
	}
	if _, err := uuid.Parse(in.MerchantID); err != nil {
		return nil, fmt.Errorf("invalid merchant_id")
	}
	outcome := strings.ToLower(strings.TrimSpace(in.OutcomeType))
	switch outcome {
	case "fulfilled", "delayed", "failed", "partial", "refunded", "disputed", "chargeback", "sla_breach", "fraud_signal":
	default:
		return nil, fmt.Errorf("invalid outcome_type")
	}

	if err := s.EnsureMerchantRole(ctx, in.MerchantID); err != nil {
		return nil, err
	}
	if err := s.UpsertMerchantProfile(ctx, in.MerchantID, "", "", "", "", nil, nil); err != nil {
		return nil, err
	}

	var buyerID *string
	if in.BuyerAgentID != "" {
		if _, err := uuid.Parse(in.BuyerAgentID); err != nil {
			return nil, fmt.Errorf("invalid buyer_agent_id")
		}
		buyerID = &in.BuyerAgentID
	}
	var orderID *string
	if in.OrderID != "" {
		orderID = &in.OrderID
	}
	var disputeRef *string
	if in.DisputeRef != "" {
		disputeRef = &in.DisputeRef
	}
	var receiptSig *string
	if in.ReceiptSignature != "" {
		receiptSig = &in.ReceiptSignature
	}

	ts := time.Now().UTC()
	if in.EventTS != nil {
		ts = in.EventTS.UTC()
	}
	if _, err := s.q.Exec(ctx, `
		INSERT INTO merchant_trust_events (merchant_id, buyer_agent_id, event_kind, outcome_type, order_id, dispute_ref, receipt_signature, event_ts, metadata)
		VALUES ($1, $2, 'objective', $3, $4, $5, $6, $7, COALESCE($8::jsonb, '{}'::jsonb))
		ON CONFLICT DO NOTHING
	`, in.MerchantID, buyerID, outcome, orderID, disputeRef, receiptSig, ts, toJSONB(in.Metadata)); err != nil {
		return nil, err
	}
	return s.Recompute(ctx, in.MerchantID)
}

func (s *Service) IngestFeedback(ctx context.Context, in FeedbackEvent) (*State, error) {
	if in.MerchantID == "" {
		return nil, fmt.Errorf("merchant_id is required")
	}
	if _, err := uuid.Parse(in.MerchantID); err != nil {
		return nil, fmt.Errorf("invalid merchant_id")
	}
	if in.Stars < 1 || in.Stars > 5 {
		return nil, fmt.Errorf("stars must be in range 1..5")
	}
	if in.SentimentScore < -1 || in.SentimentScore > 1 {
		return nil, fmt.Errorf("sentiment_score must be in range -1..1")
	}
	if in.ComplaintSeverity < 0 || in.ComplaintSeverity > 5 {
		return nil, fmt.Errorf("complaint_severity must be in range 0..5")
	}

	if err := s.EnsureMerchantRole(ctx, in.MerchantID); err != nil {
		return nil, err
	}
	if err := s.UpsertMerchantProfile(ctx, in.MerchantID, "", "", "", "", nil, nil); err != nil {
		return nil, err
	}

	weight := 1.0
	if in.BuyerAgentID != "" {
		if _, err := uuid.Parse(in.BuyerAgentID); err != nil {
			return nil, fmt.Errorf("invalid buyer_agent_id")
		}
		var cnt int
		_ = s.q.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM merchant_trust_events
			WHERE merchant_id = $1 AND buyer_agent_id::text = $2
			AND event_kind = 'feedback'
			AND event_ts > NOW() - INTERVAL '24 hours'
		`, in.MerchantID, in.BuyerAgentID).Scan(&cnt)
		if cnt >= 5 {
			weight = 0.2
		} else if cnt >= 3 {
			weight = 0.5
		}
	}

	ts := time.Now().UTC()
	if in.EventTS != nil {
		ts = in.EventTS.UTC()
	}
	_, err := s.q.Exec(ctx, `
		INSERT INTO merchant_trust_events (merchant_id, buyer_agent_id, event_kind, stars, sentiment_score, complaint_severity, order_id, event_ts, metadata)
		VALUES ($1, NULLIF($2, '')::uuid, 'feedback', $3, $4, $5, NULLIF($6, ''), $7, jsonb_build_object('weight', $8))
	`, in.MerchantID, in.BuyerAgentID, in.Stars, in.SentimentScore, in.ComplaintSeverity, in.OrderID, ts, weight)
	if err != nil {
		return nil, err
	}
	return s.Recompute(ctx, in.MerchantID)
}

func (s *Service) Recompute(ctx context.Context, merchantID string) (*State, error) {
	var objectiveAvg, feedbackAvg, feedbackWAvg float64
	var objectiveN, feedbackN int

	_ = s.q.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(
				CASE outcome_type
					WHEN 'fulfilled' THEN 1.00
					WHEN 'delayed' THEN 0.70
					WHEN 'partial' THEN 0.50
					WHEN 'failed' THEN 0.20
					WHEN 'refunded' THEN 0.15
					WHEN 'disputed' THEN 0.05
					WHEN 'chargeback' THEN 0.00
					WHEN 'sla_breach' THEN 0.10
					WHEN 'fraud_signal' THEN 0.00
					ELSE 0.50
				END
			), 0.50),
			COUNT(*)
		FROM merchant_trust_events
		WHERE merchant_id = $1 AND event_kind = 'objective'
	`, merchantID).Scan(&objectiveAvg, &objectiveN)

	_ = s.q.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(((stars::float - 1.0) / 4.0 + ((sentiment_score + 1.0) / 2.0)) / 2.0), 0.50),
			COALESCE(AVG((((stars::float - 1.0) / 4.0 + ((sentiment_score + 1.0) / 2.0)) / 2.0) *
				COALESCE((metadata->>'weight')::float, 1.0)), 0.50),
			COUNT(*)
		FROM merchant_trust_events
		WHERE merchant_id = $1 AND event_kind = 'feedback'
	`, merchantID).Scan(&feedbackAvg, &feedbackWAvg, &feedbackN)

	weightedFeedback := feedbackWAvg
	if feedbackN == 0 {
		weightedFeedback = 0.5
	}

	objectiveWeight := 0.80
	feedbackWeight := 0.20
	if objectiveN < 5 {
		objectiveWeight = 0.65
		feedbackWeight = 0.35
	}
	if math.Abs(objectiveAvg-weightedFeedback) > 0.40 {
		objectiveWeight = 0.90
		feedbackWeight = 0.10
	}

	// Time-decay makes older good history matter less than recent performance.
	decay := 0.92
	score := clamp((objectiveAvg*objectiveWeight+weightedFeedback*feedbackWeight)*decay+0.04, 0, 1)

	total := objectiveN + feedbackN
	confidence := clamp(float64(total)/100.0, 0.1, 1.0)
	volume := "low"
	if total >= 100 {
		volume = "high"
	} else if total >= 20 {
		volume = "medium"
	}

	flags := make([]string, 0)
	if confidence < 0.35 {
		flags = append(flags, "low_confidence")
	}
	if score < 0.35 {
		flags = append(flags, "low_trust")
	}
	if score < 0.2 {
		flags = append(flags, "severe_risk")
	}
	suspended := score < 0.1 && objectiveN >= 3
	hitlOnly := score < 0.35 || confidence < 0.25

	_, err := s.q.Exec(ctx, `
		INSERT INTO merchant_trust_state
			(merchant_id, trust_score, confidence, volume_bucket, risk_flags, total_objective_events, total_feedback_events, suspended, hitl_only, last_updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (merchant_id) DO UPDATE
		SET trust_score = EXCLUDED.trust_score,
			confidence = EXCLUDED.confidence,
			volume_bucket = EXCLUDED.volume_bucket,
			risk_flags = EXCLUDED.risk_flags,
			total_objective_events = EXCLUDED.total_objective_events,
			total_feedback_events = EXCLUDED.total_feedback_events,
			suspended = EXCLUDED.suspended,
			hitl_only = EXCLUDED.hitl_only,
			last_updated_at = NOW()
	`, merchantID, score, confidence, volume, flags, objectiveN, feedbackN, suspended, hitlOnly)
	if err != nil {
		return nil, err
	}
	return s.GetState(ctx, merchantID)
}

func (s *Service) GetState(ctx context.Context, merchantID string) (*State, error) {
	var st State
	err := s.q.QueryRow(ctx, `
		SELECT merchant_id::text, trust_score, confidence, volume_bucket, risk_flags,
			total_objective_events, total_feedback_events, suspended, hitl_only, last_updated_at
		FROM merchant_trust_state WHERE merchant_id = $1
	`, merchantID).Scan(
		&st.MerchantID, &st.TrustScore, &st.Confidence, &st.VolumeBucket, &st.RiskFlags,
		&st.TotalObjectiveEvents, &st.TotalFeedbackEvents, &st.Suspended, &st.HITLOnly, &st.LastUpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func toJSONB(v map[string]interface{}) interface{} {
	if v == nil {
		return "{}"
	}
	return v
}
