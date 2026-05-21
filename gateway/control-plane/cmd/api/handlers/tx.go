package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/tx"
)

var tokenService *tx.TokenService
var revocationStore *tx.RevocationStore

func SetTokenService(svc *tx.TokenService) {
	tokenService = svc
}

func SetRevocationStore(store *tx.RevocationStore) {
	revocationStore = store
}

func IssueCapabilityToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID    string                 `json:"agent_id"`
		ResourceID string                `json:"resource_id"`
		Action     string                 `json:"action"`
		Scopes     []string               `json:"scopes"`
		Params     map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.AgentID == "" || req.Action == "" {
		http.Error(w, "agent_id and action are required", http.StatusBadRequest)
		return
	}

	if tokenService == nil {
		http.Error(w, "token service not configured", http.StatusServiceUnavailable)
		return
	}

	var owner string
	var trustScore float64
	var allowedTools []string
	err := querier.QueryRow(r.Context(),
		`SELECT owner, trust_score, allowed_tools FROM agents WHERE agent_id = $1 AND status = 'active'`,
		req.AgentID,
	).Scan(&owner, &trustScore, &allowedTools)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed": false,
			"reason":  "agent not found or inactive",
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

	if req.ResourceID != "" {
		var reqSkills []string
		querier.QueryRow(r.Context(),
			`SELECT COALESCE(required_skills, '{}') FROM resources WHERE resource_id = $1`,
			req.ResourceID,
		).Scan(&reqSkills)
		if len(reqSkills) > 0 {
			skillRows, skillErr := querier.Query(r.Context(),
				`SELECT skill_id, name, COALESCE(required_proficiency, 1), COALESCE(required_trust_min, 0.5) FROM skills WHERE name = ANY($1)`,
				reqSkills,
			)
			if skillErr == nil {
				defer skillRows.Close()
				for skillRows.Next() {
					var sr policy.SkillRequirement
					if err := skillRows.Scan(&sr.SkillID, &sr.SkillName, &sr.MinProficiency, &sr.MinTrust); err == nil {
						policyInput.RequiredSkills = append(policyInput.RequiredSkills, sr)
					}
				}
			}
		}
	}

	agentSkillRows, agentSkillErr := querier.Query(r.Context(),
		`SELECT als.skill_id, s.name, als.proficiency, als.verified FROM agent_skills als JOIN skills s ON s.skill_id = als.skill_id WHERE als.agent_id = $1`,
		req.AgentID,
	)
	if agentSkillErr == nil {
		defer agentSkillRows.Close()
		for agentSkillRows.Next() {
			var ase policy.AgentSkillEntry
			if err := agentSkillRows.Scan(&ase.SkillID, &ase.SkillName, &ase.Proficiency, &ase.Verified); err == nil {
				policyInput.AgentSkills = append(policyInput.AgentSkills, ase)
			}
		}
	}

	trustRows, trustErr := querier.Query(r.Context(),
		`SELECT sts.skill_id, sts.trust_score FROM skill_trust_scores sts WHERE sts.agent_id = $1`,
		req.AgentID,
	)
	if trustErr == nil {
		defer trustRows.Close()
		for trustRows.Next() {
			var ste policy.SkillTrustEntry
			if err := trustRows.Scan(&ste.SkillID, &ste.TrustScore); err == nil {
				policyInput.SkillTrustScores = append(policyInput.SkillTrustScores, ste)
			}
		}
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

	if auditLogger != nil && gatewayPrivateKey != nil {
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
	}

	if !decision.Allowed {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed":       false,
			"requires_hitl": decision.RequiresHITL,
			"reason":        decision.Reason,
			"trust_delta":   decision.TrustDelta,
			"missing_skills": decision.MissingSkills,
		})
		return
	}

	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{req.Action}
	}

	var skillClaims []tx.SkillClaim
	for _, as := range policyInput.AgentSkills {
		skillClaims = append(skillClaims, tx.SkillClaim{
			SkillID:     as.SkillID,
			SkillName:   as.SkillName,
			Proficiency: as.Proficiency,
			Verified:    as.Verified,
		})
	}

	token, err := tokenService.IssueToken(req.AgentID, req.ResourceID, req.Action, newTrustScore, scopes, skillClaims, req.Params)
	if err != nil {
		slog.Error("issue capability token failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":         true,
		"requires_hitl":  decision.RequiresHITL,
		"reason":          decision.Reason,
		"trust_delta":    decision.TrustDelta,
		"capability_token": token,
		"missing_skills":  decision.MissingSkills,
	})
}

func VerifyCapabilityToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if tokenService == nil {
		http.Error(w, "token service not configured", http.StatusServiceUnavailable)
		return
	}

	token, err := tokenService.DecodeAndVerifyToken(req.Token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"reason": err.Error(),
		})
		return
	}

	if revocationStore != nil {
		revoked, revErr := revocationStore.IsRevoked(r.Context(), token.ID)
		if revErr != nil {
			slog.Error("revocation check failed", "error", revErr)
		}
		if revoked {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":  false,
				"reason": "token has been revoked",
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"token":   token,
		"reason":  "token is valid and not revoked",
	})
}

func RevokeCapabilityToken(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "tokenID")
	if tokenID == "" {
		http.Error(w, "token_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "revoked by administrator"
	}

	if revocationStore == nil {
		http.Error(w, "revocation store not configured", http.StatusServiceUnavailable)
		return
	}

	if err := revocationStore.RevokeToken(r.Context(), tokenID, req.Reason); err != nil {
		slog.Error("revoke token failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if auditLogger != nil && gatewayPrivateKey != nil {
		auditEntry := audit.AuditEntry{
			Action: "token.revoke",
			Method: "POST",
			Status: "success",
		}
		auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "revoked",
		"token_id": tokenID,
	})
}

func ListRevokedTokens(w http.ResponseWriter, r *http.Request) {
	if revocationStore == nil {
		http.Error(w, "revocation store not configured", http.StatusServiceUnavailable)
		return
	}

	tokens, err := revocationStore.ListRevokedTokens(r.Context(), 50)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if tokens == nil {
		tokens = []tx.RevokedToken{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"revoked_tokens": tokens,
	})
}

func IssueTransactionReceipt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string               `json:"token"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if tokenService == nil {
		http.Error(w, "token service not configured", http.StatusServiceUnavailable)
		return
	}

	token, err := tokenService.DecodeAndVerifyToken(req.Token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"reason": err.Error(),
		})
		return
	}

	if revocationStore != nil {
		revoked, _ := revocationStore.IsRevoked(r.Context(), token.ID)
		if revoked {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":  false,
				"reason": "token has been revoked",
			})
			return
		}
	}

	receipt, err := tokenService.IssueReceipt(token, true, token.TrustScore, 0.01)
	if err != nil {
		slog.Error("issue receipt failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if auditLogger != nil && gatewayPrivateKey != nil {
		auditEntry := audit.AuditEntry{
			AgentID:    token.Subject,
			ResourceID: token.ResourceID,
			Action:     token.Action,
			Method:     "POST",
			Status:     "receipt_issued",
		}
		auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"receipt": receipt,
	})
}

func VerifyTransactionReceipt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Receipt string `json:"receipt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Receipt == "" {
		http.Error(w, "receipt is required", http.StatusBadRequest)
		return
	}

	if tokenService == nil {
		http.Error(w, "token service not configured", http.StatusServiceUnavailable)
		return
	}

	var receipt tx.TransactionReceipt
	if err := json.Unmarshal([]byte(req.Receipt), &receipt); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"reason": "invalid receipt format",
		})
		return
	}

	if err := tokenService.VerifyReceipt(&receipt); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"reason": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"receipt": receipt,
		"reason":  "receipt signature is valid",
	})
}