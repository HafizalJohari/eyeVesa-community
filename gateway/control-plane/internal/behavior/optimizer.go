package behavior

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultAnomalyThreshold = 0.7
	anomalyTrustDelta       = -0.05
)

type BehaviorOptimizer struct {
	db        *pgxpool.Pool
	embedder  *EmbeddingService
	threshold float64
}

func NewBehaviorOptimizer(db *pgxpool.Pool, embedder *EmbeddingService) *BehaviorOptimizer {
	return &BehaviorOptimizer{
		db:        db,
		embedder:  embedder,
		threshold: defaultAnomalyThreshold,
	}
}

// AnalyzeAnomalies compares the newest successful action against the agent's
// existing baseline vector, records drift, applies a trust markdown, then updates
// the baseline so future decisions learn from successful behavior.
func (bo *BehaviorOptimizer) AnalyzeAnomalies(ctx context.Context, agentID string, currentAction string, params map[string]interface{}) (float64, error) {
	if bo == nil || bo.db == nil || bo.embedder == nil {
		return 1.0, nil
	}

	if err := bo.embedder.RecordEvent(ctx, agentID, currentAction, "", "success", params); err != nil {
		return 1.0, fmt.Errorf("record behavior event: %w", err)
	}

	currentVec, err := bo.embedder.GenerateEmbedding(ctx, agentID)
	if err != nil {
		return 1.0, fmt.Errorf("generate current behavior vector: %w", err)
	}

	var similarity float64
	err = bo.db.QueryRow(ctx,
		`SELECT 1 - (behavior_vec <=> $1::vector) AS similarity
		 FROM agents
		 WHERE agent_id = $2 AND behavior_vec IS NOT NULL`,
		formatVector(currentVec), agentID,
	).Scan(&similarity)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1.0, bo.embedder.UpdateAgentEmbedding(ctx, agentID)
	}
	if err != nil {
		return 1.0, fmt.Errorf("compare behavior baseline: %w", err)
	}

	if similarity < bo.threshold {
		if err := bo.recordAnomaly(ctx, agentID, currentAction, similarity); err != nil {
			return similarity, err
		}
	}

	return similarity, bo.embedder.UpdateAgentEmbedding(ctx, agentID)
}

func (bo *BehaviorOptimizer) recordAnomaly(ctx context.Context, agentID string, action string, similarity float64) error {
	tx, err := bo.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	anomalyID := uuid.New()
	if _, err := tx.Exec(ctx,
		`INSERT INTO behavioral_anomalies (anomaly_id, agent_id, similarity_score, baseline_behavior, detected_behavior, anomaly_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		anomalyID, agentID, similarity, "historical behavior_vec baseline",
		fmt.Sprintf("action %q similarity %.4f", action, similarity), "self_improving_drift",
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE agents
		 SET trust_score = GREATEST(0, LEAST(1, trust_score + $1)), updated_at = NOW()
		 WHERE agent_id = $2`,
		anomalyTrustDelta, agentID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO trust_events (agent_id, event_type, trust_delta, trust_score_after, reason)
		 SELECT agent_id, 'behavioral_drift', $1, trust_score, $2
		 FROM agents WHERE agent_id = $3`,
		anomalyTrustDelta, fmt.Sprintf("behavioral similarity %.4f below %.2f for action %s", similarity, bo.threshold, action), agentID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
