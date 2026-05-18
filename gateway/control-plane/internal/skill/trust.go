package skill

import (
	"context"
	"fmt"
	"math"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

type SkillTrustTracker struct {
	q database.Querier
}

func NewSkillTrustTracker(q database.Querier) *SkillTrustTracker {
	return &SkillTrustTracker{q: q}
}

type TrustAdjustment struct {
	AgentID  string
	SkillID  string
	Delta    float64
	Reason   string
	NewScore float64
}

func (t *SkillTrustTracker) AdjustSkillTrust(ctx context.Context, agentID, skillID string, delta float64, reason string) (*TrustAdjustment, error) {
	service := &SkillService{q: t.q}
	newScore, err := service.UpdateSkillTrust(ctx, agentID, skillID, delta)
	if err != nil {
		return nil, fmt.Errorf("adjust skill trust: %w", err)
	}

	_, _ = t.q.Exec(ctx,
		`INSERT INTO trust_events (agent_id, event_type, trust_delta, trust_score_after, reason)
		 VALUES ($1::uuid, 'skill_trust', $2, $3, $4)`,
		agentID, delta, newScore, reason,
	)

	return &TrustAdjustment{
		AgentID:  agentID,
		SkillID:  skillID,
		Delta:    delta,
		Reason:   reason,
		NewScore: newScore,
	}, nil
}

func (t *SkillTrustTracker) AdjustOnEndorsement(ctx context.Context, agentID, skillID string) (*TrustAdjustment, error) {
	return t.AdjustSkillTrust(ctx, agentID, skillID, 0.02, "skill endorsement received")
}

func (t *SkillTrustTracker) AdjustOnVerification(ctx context.Context, agentID, skillID string) (*TrustAdjustment, error) {
	return t.AdjustSkillTrust(ctx, agentID, skillID, 0.05, "skill verified")
}

func (t *SkillTrustTracker) AdjustOnAuthorization(ctx context.Context, agentID, skillID string, allowed bool) (*TrustAdjustment, error) {
	delta := 0.01
	reason := "skill-based authorization: allowed"
	if !allowed {
		delta = -0.05
		reason = "skill-based authorization: denied"
	}
	return t.AdjustSkillTrust(ctx, agentID, skillID, delta, reason)
}

func ClampTrust(score float64) float64 {
	return math.Max(0, math.Min(1, score))
}