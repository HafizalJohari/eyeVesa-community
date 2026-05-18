package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

type AuthorizeRequest struct {
	AgentID    string                 `json:"agent_id"`
	ResourceID string                `json:"resource_id"`
	Action     string                 `json:"action"`
	Params     map[string]interface{} `json:"params"`
}

type AuthorizeResponse struct {
	Allowed      bool    `json:"allowed"`
	RequiresHITL bool   `json:"requires_hitl"`
	Reason       string  `json:"reason"`
	TrustDelta   float64 `json:"trust_delta"`
}

func Authorize(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Action == "" {
		http.Error(w, "agent_id and action are required", http.StatusBadRequest)
		return
	}

	var owner string
	var trustScore float64
	var capabilities, allowedTools []string
	err := querier.QueryRow(r.Context(),
		`SELECT owner, trust_score, capabilities, allowed_tools FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.AgentID,
	).Scan(&owner, &trustScore, &capabilities, &allowedTools)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AuthorizeResponse{
			Allowed: false,
			Reason:  "agent not found or inactive",
		})
		return
	}

	policyInput := policy.PolicyInput{}
	policyInput.Agent.ID = req.AgentID
	policyInput.Agent.Owner = owner
	policyInput.Agent.TrustScore = trustScore
	policyInput.Agent.AllowedTools = allowedTools
	policyInput.Action.Tool = req.Action
	policyInput.Action.ResourceID = req.ResourceID
	policyInput.Action.Params = req.Params
	if cost, ok := req.Params["estimated_cost"].(float64); ok {
		policyInput.Action.EstimatedCost = cost
	}

	decision := globalPolicyEngine.Evaluate(r.Context(), policyInput)

	newTrustScore := trustScore + decision.TrustDelta
	if newTrustScore < 0 {
		newTrustScore = 0
	}
	if newTrustScore > 1 {
		newTrustScore = 1
	}

	querier.Exec(r.Context(),
		`UPDATE agents SET trust_score = $1, updated_at = NOW() WHERE agent_id = $2`,
		newTrustScore, req.AgentID,
	)

	querier.Exec(r.Context(),
		`INSERT INTO trust_events (agent_id, event_type, trust_delta, trust_score_after, reason) VALUES ($1, $2, $3, $4, $5)`,
		req.AgentID, "authorize", decision.TrustDelta, newTrustScore, decision.Reason,
	)

	auditEntry := audit.AuditEntry{
		AgentID:     req.AgentID,
		ResourceID:  req.ResourceID,
		Action:      req.Action,
		Method:      "POST",
		Status:      map[bool]string{true: "allowed", false: "denied"}[decision.Allowed],
		TrustBefore: trustScore,
		TrustAfter:  newTrustScore,
	}
	auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)

	resp := AuthorizeResponse{
		Allowed:      decision.Allowed,
		RequiresHITL: decision.RequiresHITL,
		Reason:       decision.Reason,
		TrustDelta:   decision.TrustDelta,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func VerifySignature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID   string `json:"agent_id"`
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var pubKeyBytes []byte
	err := querier.QueryRow(r.Context(),
		`SELECT public_key FROM agents WHERE agent_id = $1`,
		req.AgentID,
	).Scan(&pubKeyBytes)

	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	if len(pubKeyBytes) == 0 {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	sig, err := crypto.DecodeBase64(req.Signature)
	if err != nil {
		http.Error(w, "invalid signature format", http.StatusBadRequest)
		return
	}

	valid := crypto.VerifySignature(pubKeyBytes, []byte(req.Message), sig)

	w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id": req.AgentID,
			"valid":    valid,
	})
}

// GetAuditLog returns audit trail for an agent
func GetAuditLog(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	pool := db.Pool
	rows, err := pool.Query(r.Context(),
		`SELECT log_id, agent_id, COALESCE(resource_id, '00000000-0000-0000-0000-000000000000'::uuid), action, method, params, result, result_status, trust_score_before, trust_score_after, session_id, signature, created_at
		 FROM audit_logs WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		agentID, int32(limit), int32(offset),
	)
	if err != nil {
		http.Error(w, "failed to query audit logs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []map[string]interface{}
	for rows.Next() {
		var logID, agID, resID, action, method, status string
		var sessionID *string
		var trustBefore, trustAfter float64
		var paramsJSON, resultJSON, signature []byte
		var createdAt time.Time
		if err := rows.Scan(&logID, &agID, &resID, &action, &method, &paramsJSON, &resultJSON, &status, &trustBefore, &trustAfter, &sessionID, &signature, &createdAt); err != nil {
			continue
		}

		var params, result map[string]interface{}
		if len(paramsJSON) > 0 {
			json.Unmarshal(paramsJSON, &params)
		}
		if params == nil {
			params = make(map[string]interface{})
		}
		if len(resultJSON) > 0 {
			json.Unmarshal(resultJSON, &result)
		}
		if result == nil {
			result = make(map[string]interface{})
		}

		sid := ""
		if sessionID != nil {
			sid = *sessionID
		}

		sig := ""
		if len(signature) > 0 {
			sig = fmt.Sprintf("%x", signature)
		}

		entries = append(entries, map[string]interface{}{
			"log_id":            logID,
			"agent_id":          agID,
			"resource_id":       resID,
			"action":            action,
			"method":            method,
			"params":            params,
			"result":            result,
			"result_status":     status,
			"trust_score_before": trustBefore,
			"trust_score_after":  trustAfter,
			"session_id":        sid,
			"signature":         sig,
			"created_at":        createdAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"entries":  entries,
		"limit":    limit,
		"offset":   offset,
	})
}
