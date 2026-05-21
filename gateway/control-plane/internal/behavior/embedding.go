package behavior

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/llm"
)

type EmbeddingService struct {
	db        *pgxpool.Pool
	llm       *llm.LLMService
	vecDim    int
}

func NewEmbeddingService(db *pgxpool.Pool, llmService *llm.LLMService) *EmbeddingService {
	return &EmbeddingService{
		db:     db,
		llm:    llmService,
		vecDim: 1536,
	}
}

type BehavioralAnomaly struct {
	AnomalyID          string  `json:"anomaly_id"`
	AgentID           string  `json:"agent_id"`
	SimilarityScore   float64 `json:"similarity_score"`
	BaselineBehavior string  `json:"baseline_behavior"`
	DetectedBehavior string  `json:"detected_behavior"`
	AnomalyType       string  `json:"anomaly_type"`
}

func (s *EmbeddingService) RecordEvent(ctx context.Context, agentID string, tool string, resourceID string, outcome string, params map[string]interface{}) error {
	paramsJSON := "{}"
	if params != nil {
		if b, err := json.Marshal(params); err == nil {
			paramsJSON = string(b)
		}
	}

	var paramsHash string
	if len(paramsJSON) > 64 {
		hash := sha256.Sum256([]byte(paramsJSON))
		paramsHash = fmt.Sprintf("%x", hash)[:64]
	} else {
		paramsHash = paramsJSON
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO behavioral_events (agent_id, tool, resource_id, action_outcome, params_hash)
		 VALUES ($1, $2, $3, $4, $5)`,
		agentID, tool, nilIfEmpty(resourceID), outcome, paramsHash,
	)
	return err
}

func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, agentID string) ([]float32, error) {
	rows, err := s.db.Query(ctx,
		`SELECT tool, action_outcome, COUNT(*) as cnt
		 FROM behavioral_events
		 WHERE agent_id = $1 AND created_at > NOW() - INTERVAL '7 days'
		 GROUP BY tool, action_outcome
		 ORDER BY cnt DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query behavioral events: %w", err)
	}
	defer rows.Close()

	vec := make([]float32, s.vecDim)
	toolCounts := make(map[string]float32)
	outcomeCounts := make(map[string]float32)
	totalEvents := float32(0)

	for rows.Next() {
		var tool, outcome string
		var cnt int
		if err := rows.Scan(&tool, &outcome, &cnt); err != nil {
			continue
		}
		toolCounts[tool] += float32(cnt)
		outcomeCounts[outcome] += float32(cnt)
		totalEvents += float32(cnt)
	}

	if totalEvents == 0 {
		return vec, nil
	}

	idx := 0
	for _, count := range toolCounts {
		if idx < s.vecDim {
			vec[idx] = count / totalEvents
			idx++
		}
	}
	for _, count := range outcomeCounts {
		if idx < s.vecDim {
			vec[idx] = count / totalEvents
			idx++
		}
	}

	for i := range vec {
		if math.IsNaN(float64(vec[i])) || math.IsInf(float64(vec[i]), 0) {
			vec[i] = 0
		}
	}

	return vec, nil
}

func (s *EmbeddingService) UpdateAgentEmbedding(ctx context.Context, agentID string) error {
	vec, err := s.GenerateEmbedding(ctx, agentID)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	vecBytes, err := json.Marshal(vec)
	if err != nil {
		return fmt.Errorf("marshal vector: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`UPDATE agents SET behavior_vec = $1::vector, updated_at = NOW() WHERE agent_id = $2`,
		string(vecBytes), agentID,
	)
	if err != nil {
		return fmt.Errorf("update behavior_vec: %w", err)
	}

	return nil
}

func (s *EmbeddingService) DetectAnomalies(ctx context.Context, agentID string, threshold float64) ([]BehavioralAnomaly, error) {
	vec, err := s.GenerateEmbedding(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("generate current embedding: %w", err)
	}

	err = s.UpdateAgentEmbedding(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("update agent embedding: %w", err)
	}

	rows, err := s.db.Query(ctx,
		`SELECT agent_id, behavior_vec <=> $1::vector AS distance, name
		 FROM agents
		 WHERE agent_id != $2 AND behavior_vec IS NOT NULL
		 ORDER BY behavior_vec <=> $1::vector
		 LIMIT 10`,
		formatVector(vec), agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("similarity query: %w", err)
	}
	defer rows.Close()

	var anomalies []BehavioralAnomaly
	for rows.Next() {
		var otherID, otherName string
		var distance float64
		if err := rows.Scan(&otherID, &distance, &otherName); err != nil {
			continue
		}

		similarity := 1.0 - distance
		if similarity < threshold {
			anomaly := BehavioralAnomaly{
				AgentID:          agentID,
				SimilarityScore:  similarity,
				BaselineBehavior: fmt.Sprintf("similar to agent %s", otherName),
				DetectedBehavior: fmt.Sprintf("drift score: %.4f", distance),
				AnomalyType:      "behavioral_drift",
			}

			anomalyID := uuid.New()
			anomaly.AnomalyID = anomalyID.String()

			_, err := s.db.Exec(ctx,
				`INSERT INTO behavioral_anomalies (anomaly_id, agent_id, similarity_score, baseline_behavior, detected_behavior, anomaly_type)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				anomalyID, agentID, similarity, anomaly.BaselineBehavior, anomaly.DetectedBehavior, anomaly.AnomalyType,
			)
			if err == nil {
				anomalies = append(anomalies, anomaly)
			}
		}
	}

	return anomalies, nil
}

func (s *EmbeddingService) GetAnomalies(ctx context.Context, agentID string, resolved bool) ([]BehavioralAnomaly, error) {
	query := `SELECT anomaly_id, agent_id, similarity_score, baseline_behavior, detected_behavior, anomaly_type
			  FROM behavioral_anomalies WHERE agent_id = $1 AND resolved = $2 ORDER BY created_at DESC`
	rows, err := s.db.Query(ctx, query, agentID, resolved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []BehavioralAnomaly
	for rows.Next() {
		var a BehavioralAnomaly
		if err := rows.Scan(&a.AnomalyID, &a.AgentID, &a.SimilarityScore, &a.BaselineBehavior, &a.DetectedBehavior, &a.AnomalyType); err != nil {
			continue
		}
		anomalies = append(anomalies, a)
	}
	return anomalies, nil
}

func (s *EmbeddingService) ResolveAnomaly(ctx context.Context, anomalyID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE behavioral_anomalies SET resolved = TRUE WHERE anomaly_id = $1`,
		anomalyID,
	)
	return err
}

func (s *EmbeddingService) GetSimilarAgents(ctx context.Context, agentID string, limit int) ([]map[string]interface{}, error) {
	var vecStr string
	err := s.db.QueryRow(ctx,
		`SELECT behavior_vec::text FROM agents WHERE agent_id = $1 AND behavior_vec IS NOT NULL`,
		agentID,
	).Scan(&vecStr)
	if err != nil {
		return nil, fmt.Errorf("agent has no behavior embedding")
	}

	rows, err := s.db.Query(ctx,
		`SELECT agent_id, name, trust_score, behavior_vec <=> $1::vector AS distance
		 FROM agents
		 WHERE agent_id != $2 AND behavior_vec IS NOT NULL
		 ORDER BY behavior_vec <=> $1::vector
		 LIMIT $3`,
		vecStr, agentID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, name string
		var trustScore float64
		var distance float64
		if err := rows.Scan(&id, &name, &trustScore, &distance); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"agent_id":    id,
			"name":        name,
			"trust_score": trustScore,
			"similarity":  1.0 - distance,
		})
	}
	return results, nil
}

func formatVector(vec []float32) string {
	str := "["
	for i, v := range vec {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf("%f", v)
	}
	str += "]"
	return str
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}