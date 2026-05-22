package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/merchanttrust"
)

var merchantTrustService *merchanttrust.Service

func SetMerchantTrustService(svc *merchanttrust.Service) {
	merchantTrustService = svc
}

type MerchantProfileRequest struct {
	MerchantID       string   `json:"merchant_id"`
	BusinessType     string   `json:"business_type"`
	Categories       []string `json:"categories"`
	FulfillmentModel string   `json:"fulfillment_model"`
	Regions          []string `json:"regions"`
	SupportSLA       string   `json:"support_sla"`
	VerificationTier string   `json:"verification_tier"`
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func CreateMerchantProfile(w http.ResponseWriter, r *http.Request) {
	if merchantTrustService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "merchant trust service not configured")
		return
	}
	var req MerchantProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MerchantID == "" {
		writeJSONError(w, http.StatusBadRequest, "merchant_id is required")
		return
	}
	var exists int
	if err := querier.QueryRow(r.Context(), `SELECT 1 FROM agents WHERE agent_id::text = $1`, req.MerchantID).Scan(&exists); err != nil || exists != 1 {
		writeJSONError(w, http.StatusNotFound, "merchant agent not found; register agent first")
		return
	}
	if err := merchantTrustService.EnsureMerchantRole(r.Context(), req.MerchantID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to assign merchant role")
		return
	}
	if err := merchantTrustService.UpsertMerchantProfile(r.Context(), req.MerchantID, req.BusinessType, req.FulfillmentModel, req.SupportSLA, req.VerificationTier, req.Categories, req.Regions); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to upsert merchant profile")
		return
	}
	st, err := merchantTrustService.GetState(r.Context(), req.MerchantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "merchant profile created but trust state unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"merchant_id": req.MerchantID, "trust": st})
}

func ListMerchants(w http.ResponseWriter, r *http.Request) {
	rows, err := querier.Query(r.Context(), `
		SELECT a.agent_id::text, a.name, a.owner,
			mp.business_type, mp.categories, mp.verification_tier,
			COALESCE(mts.trust_score, 0.5), COALESCE(mts.confidence, 0.1),
			COALESCE(mts.risk_flags, '{}'::text[]), COALESCE(mts.hitl_only, false), COALESCE(mts.suspended, false)
		FROM agents a
		JOIN merchant_profiles mp ON mp.merchant_id = a.agent_id
		LEFT JOIN merchant_trust_state mts ON mts.merchant_id = a.agent_id
		ORDER BY COALESCE(mts.trust_score, 0.5) DESC
		LIMIT 200
	`)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, name, owner, bType, tier string
		var categories, riskFlags []string
		var trust, confidence float64
		var hitlOnly, suspended bool
		if err := rows.Scan(&id, &name, &owner, &bType, &categories, &tier, &trust, &confidence, &riskFlags, &hitlOnly, &suspended); err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"merchant_id": id, "name": name, "owner": owner, "business_type": bType,
			"categories": categories, "verification_tier": tier, "trust_score": trust,
			"confidence": confidence, "risk_flags": riskFlags, "hitl_only": hitlOnly, "suspended": suspended,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"merchants": out, "count": len(out)})
}

func GetMerchant(w http.ResponseWriter, r *http.Request) {
	merchantID := chi.URLParam(r, "merchantID")
	if merchantID == "" {
		writeJSONError(w, http.StatusBadRequest, "merchantID required")
		return
	}
	var name, owner, bType, fulfillment, supportSLA, tier string
	var categories, regions []string
	err := querier.QueryRow(r.Context(), `
		SELECT a.name, a.owner, mp.business_type, mp.categories, mp.fulfillment_model, mp.regions, mp.support_sla, mp.verification_tier
		FROM agents a JOIN merchant_profiles mp ON mp.merchant_id = a.agent_id
		WHERE a.agent_id::text = $1
	`, merchantID).Scan(&name, &owner, &bType, &categories, &fulfillment, &regions, &supportSLA, &tier)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "merchant not found")
		return
	}
	trust, _ := merchantTrustService.GetState(r.Context(), merchantID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"merchant_id": merchantID, "name": name, "owner": owner, "business_type": bType,
		"categories": categories, "fulfillment_model": fulfillment, "regions": regions,
		"support_sla": supportSLA, "verification_tier": tier, "trust": trust,
	})
}

func GetMerchantTrust(w http.ResponseWriter, r *http.Request) {
	if merchantTrustService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "merchant trust service not configured")
		return
	}
	merchantID := chi.URLParam(r, "merchantID")
	if merchantID == "" {
		writeJSONError(w, http.StatusBadRequest, "merchantID required")
		return
	}
	st, err := merchantTrustService.GetState(r.Context(), merchantID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "merchant trust not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func IngestMerchantOutcomeEvent(w http.ResponseWriter, r *http.Request) {
	if merchantTrustService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "merchant trust service not configured")
		return
	}
	var req merchanttrust.OutcomeEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var exists int
	if err := querier.QueryRow(r.Context(), `SELECT 1 FROM agents WHERE agent_id::text = $1`, req.MerchantID).Scan(&exists); err != nil || exists != 1 {
		writeJSONError(w, http.StatusNotFound, "merchant agent not found; register agent first")
		return
	}
	st, err := merchantTrustService.IngestOutcome(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "trust": st})
}

func IngestMerchantFeedbackEvent(w http.ResponseWriter, r *http.Request) {
	if merchantTrustService == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "merchant trust service not configured")
		return
	}
	var req merchanttrust.FeedbackEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var exists int
	if err := querier.QueryRow(r.Context(), `SELECT 1 FROM agents WHERE agent_id::text = $1`, req.MerchantID).Scan(&exists); err != nil || exists != 1 {
		writeJSONError(w, http.StatusNotFound, "merchant agent not found; register agent first")
		return
	}
	st, err := merchantTrustService.IngestFeedback(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "trust": st})
}
