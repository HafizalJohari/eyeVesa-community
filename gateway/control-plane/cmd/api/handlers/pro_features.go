package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/behavior"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/hitl"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/llm"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/tenant"
)

var escalationService *hitl.EscalationService
var llmService *llm.LLMService
var embeddingService *behavior.EmbeddingService
var tenantService *tenant.TenantService
var pushService *hitl.PushService

func SetEscalationService(s *hitl.EscalationService) {
	escalationService = s
}
func SetLLMService(s *llm.LLMService) {
	llmService = s
}
func SetEmbeddingService(s *behavior.EmbeddingService) {
	embeddingService = s
}
func SetTenantService(s *tenant.TenantService) {
	tenantService = s
}
func SetPushService(s *hitl.PushService) {
	pushService = s
}

func RequestEscalatedApproval(w http.ResponseWriter, r *http.Request) {
	var req hitl.ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Action == "" {
		http.Error(w, "agent_id and action are required", http.StatusBadRequest)
		return
	}

	resp, err := escalationService.RequestEscalatedApproval(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.HasPrefix(err.Error(), "auto-deny") {
			status = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "approval_denied",
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Status == "auto_allowed" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"approval_id": resp.ApprovalID,
			"status":      "auto_allowed",
			"message":      "action auto-allowed by policy",
		})
	} else {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}
}

func ProcessChainDecision(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approval_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		ApproverID string `json:"approver_id"`
		Approved   bool   `json:"approved"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ApproverID == "" {
		http.Error(w, "approver_id required", http.StatusBadRequest)
		return
	}

	entry, err := escalationService.ProcessChainDecision(r.Context(), approvalID, req.ApproverID, req.Approved, req.Reason)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func GetApprovalChain(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approval_id required", http.StatusBadRequest)
		return
	}

	entries, err := escalationService.GetApprovalChain(r.Context(), approvalID)
	if err != nil {
		http.Error(w, "failed to get chain", http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []hitl.ApprovalChainEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chain": entries,
	})
}

func GetNotifications(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approval_id required", http.StatusBadRequest)
		return
	}

	entries, err := escalationService.GetNotifications(r.Context(), approvalID)
	if err != nil {
		http.Error(w, "failed to get notifications", http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []hitl.NotificationEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": entries,
	})
}

func GenerateHITLSummary(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approval_id required", http.StatusBadRequest)
		return
	}

	var agent struct {
		Name       string  `json:"agent_name"`
		TrustScore float64 `json:"trust_score"`
	}
	var hitlApproval struct {
		Action     string                 `json:"action"`
		ResourceID string                 `json:"resource_id"`
		RiskLevel  string                 `json:"risk_level"`
		Params     map[string]interface{} `json:"params"`
	}

	err := querier.QueryRow(r.Context(),
		`SELECT name, trust_score FROM agents WHERE agent_id = (SELECT agent_id FROM hitl_approvals WHERE approval_id = $1)`,
		approvalID,
	).Scan(&agent.Name, &agent.TrustScore)
	if err != nil {
		http.Error(w, "approval not found", http.StatusNotFound)
		return
	}

	_ = querier.QueryRow(r.Context(),
		`SELECT action, COALESCE(resource_id::text, ''), COALESCE(risk_level, 'medium'), COALESCE(params::text, '{}') FROM hitl_approvals WHERE approval_id = $1`,
		approvalID,
	).Scan(&hitlApproval.Action, &hitlApproval.ResourceID, &hitlApproval.RiskLevel, &hitlApproval.Params)

	req := llm.HITLSummaryRequest{
		AgentID:     approvalID,
		AgentName:   agent.Name,
		TrustScore:  agent.TrustScore,
		Action:      hitlApproval.Action,
		ResourceID:  hitlApproval.ResourceID,
		RiskLevel:   hitlApproval.RiskLevel,
		Params:      hitlApproval.Params,
	}

	summary, err := llmService.GenerateHITLSummary(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	summary.ApprovalID = approvalID

	if querier != nil {
		querier.Exec(r.Context(),
			`INSERT INTO hitl_summaries (approval_id, summary_text, recommendation, model_used, tokens_used)
			 VALUES ($1, $2, $3, $4, $5)`,
			approvalID, summary.Summary, summary.Recommendation, summary.ModelUsed, summary.TokensUsed,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func GenerateAuditNarrative(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		Days    int    `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Days == 0 {
		req.Days = 7
	}

	periodStart := "NOW() - INTERVAL '1 day'"
	if req.Days > 1 {
		periodStart = fmt.Sprintf("NOW() - INTERVAL '%d days'", req.Days)
	}
	_ = periodStart

	var agentName string
	_ = querier.QueryRow(r.Context(),
		`SELECT name FROM agents WHERE agent_id = $1`, req.AgentID,
	).Scan(&agentName)

	rows, err := querier.Query(r.Context(),
		`SELECT action, result_status, trust_delta, created_at FROM audit_logs
		 WHERE agent_id = (SELECT name FROM agents WHERE agent_id = $1)
		 AND created_at > NOW() - $2::interval
		 ORDER BY created_at DESC LIMIT 50`,
		req.AgentID, fmt.Sprintf("%d days", req.Days),
	)
	if err != nil {
		http.Error(w, "failed to query audit logs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []llm.AuditEvent
	for rows.Next() {
		var e llm.AuditEvent
		if err := rows.Scan(&e.Action, &e.Status, &e.TrustDelta, &e.Timestamp); err != nil {
			continue
		}
		events = append(events, e)
	}

	llmReq := llm.AuditNarrativeRequest{
		AgentID:     req.AgentID,
		AgentName:   agentName,
		PeriodEnd:   time.Now(),
		Events:      events,
	}
	llmReq.PeriodStart = time.Now().AddDate(0, 0, -req.Days)

	narrative, err := llmService.GenerateAuditNarrative(r.Context(), llmReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	querier.Exec(r.Context(),
		`INSERT INTO audit_narratives (agent_id, period_start, period_end, narrative_text, key_events, anomalies_detected, trust_trend, model_used, tokens_used)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		req.AgentID, llmReq.PeriodStart, llmReq.PeriodEnd,
		narrative.NarrativeText, narrative.KeyEvents, narrative.AnomaliesDetected,
		narrative.TrustTrend, narrative.ModelUsed, narrative.TokensUsed,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(narrative)
}

func TranslatePolicy(w http.ResponseWriter, r *http.Request) {
	var req llm.PolicyTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NaturalLanguage == "" {
		http.Error(w, "natural_language is required", http.StatusBadRequest)
		return
	}

	result, err := llmService.TranslatePolicy(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if querier != nil {
		querier.Exec(r.Context(),
			`INSERT INTO policy_translations (natural_language, generated_rego, status, validated, model_used, tokens_used)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			req.NaturalLanguage, result.GeneratedRego, result.Status, result.Validated, result.ModelUsed, result.TokensUsed,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func DetectBehavioralAnomalies(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	threshold := 0.7
	if t := r.URL.Query().Get("threshold"); t != "" {
		if parsed, err := strconv.ParseFloat(t, 64); err == nil {
			threshold = parsed
		}
	}

	anomalies, err := embeddingService.DetectAnomalies(r.Context(), agentID, threshold)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if anomalies == nil {
		anomalies = []behavior.BehavioralAnomaly{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":   agentID,
		"anomalies":  anomalies,
		"threshold":  threshold,
	})
}

func GetSimilarAgents(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	similar, err := embeddingService.GetSimilarAgents(r.Context(), agentID, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if similar == nil {
		similar = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"similar":  similar,
	})
}

func UpdateBehaviorEmbedding(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	err := embeddingService.UpdateAgentEmbedding(r.Context(), agentID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"status":   "embedding_updated",
	})
}

func CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Slug         string `json:"slug"`
		Plan         string `json:"plan"`
		MaxAgents    int    `json:"max_agents"`
		MaxResources int    `json:"max_resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Slug == "" {
		http.Error(w, "name and slug are required", http.StatusBadRequest)
		return
	}

	if req.Plan == "" {
		req.Plan = "community"
	}
	if req.MaxAgents == 0 {
		req.MaxAgents = 5
	}
	if req.MaxResources == 0 {
		req.MaxResources = 10
	}

	t, err := tenantService.CreateTenant(r.Context(), req.Name, req.Slug, req.Plan, req.MaxAgents, req.MaxResources)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}

	t, err := tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := tenantService.ListTenants(r.Context())
	if err != nil {
		http.Error(w, "failed to list tenants", http.StatusInternalServerError)
		return
	}

	if tenants == nil {
		tenants = []tenant.Tenant{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tenants": tenants,
	})
}

func CheckBudget(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	estimatedCost := 0.0
	if ec := r.URL.Query().Get("estimated_cost"); ec != "" {
		if parsed, err := strconv.ParseFloat(ec, 64); err == nil {
			estimatedCost = parsed
		}
	}

	allowed, remaining, err := escalationService.CheckBudget(r.Context(), agentID, estimatedCost)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":       agentID,
		"allowed":        allowed,
		"remaining_budget": remaining,
		"estimated_cost": estimatedCost,
	})
}

func RecordSpend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID       string  `json:"agent_id"`
		ResourceID    string  `json:"resource_id"`
		Action        string  `json:"action"`
		EstimatedCost float64 `json:"estimated_cost"`
		ActualCost    float64 `json:"actual_cost"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Action == "" {
		http.Error(w, "agent_id and action required", http.StatusBadRequest)
		return
	}

	err := escalationService.RecordSpend(r.Context(), req.AgentID, req.ResourceID, req.Action, req.EstimatedCost, req.ActualCost)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "recorded",
	})
}

func RegisterPushToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ApproverID  string `json:"approver_id"`
		DeviceToken string `json:"device_token"`
		Platform    string `json:"platform"`
		BundleID    string `json:"bundle_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ApproverID == "" || req.DeviceToken == "" {
		http.Error(w, "approver_id and device_token are required", http.StatusBadRequest)
		return
	}

	if req.Platform == "" {
		req.Platform = "ios"
	}

	token, err := pushService.RegisterToken(r.Context(), req.ApproverID, req.DeviceToken, req.Platform, req.BundleID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(token)
}

func GetPushTokens(w http.ResponseWriter, r *http.Request) {
	approverID := r.URL.Query().Get("approver_id")
	if approverID == "" {
		http.Error(w, "approver_id required", http.StatusBadRequest)
		return
	}

	tokens, err := pushService.GetTokensForApprover(r.Context(), approverID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if tokens == nil {
		tokens = []hitl.PushToken{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tokens": tokens,
	})
}

func DeactivatePushToken(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "tokenID")
	if tokenID == "" {
		http.Error(w, "token_id required", http.StatusBadRequest)
		return
	}

	err := pushService.DeactivateToken(r.Context(), tokenID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "deactivated",
		"token_id": tokenID,
	})
}