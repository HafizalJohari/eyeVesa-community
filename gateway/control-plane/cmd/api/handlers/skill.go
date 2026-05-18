package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/skill"
)

func CreateSkill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                string  `json:"name"`
		Description         string  `json:"description"`
		Category            string  `json:"category"`
		RiskLevel           string  `json:"risk_level"`
		RequiredTrustMin    float64 `json:"required_trust_min"`
		RequiredProficiency int     `json:"required_proficiency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	sk, err := svc.CreateSkill(r.Context(), req.Name, req.Description, req.Category, req.RiskLevel, req.RequiredTrustMin, req.RequiredProficiency)
	if err != nil {
		slog.Error("create skill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sk)
}

func GetSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")
	if skillID == "" {
		http.Error(w, "skill_id required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	sk, err := svc.GetSkill(r.Context(), skillID)
	if err != nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sk)
}

func ListSkills(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	svc := skill.NewSkillServiceWithQuerier(querier)
	skills, err := svc.ListSkills(r.Context(), category)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if skills == nil {
		skills = []skill.Skill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
	})
}

func UpdateSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")
	if skillID == "" {
		http.Error(w, "skill_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		Description         string  `json:"description"`
		Category            string  `json:"category"`
		RiskLevel           string  `json:"risk_level"`
		RequiredTrustMin    float64 `json:"required_trust_min"`
		RequiredProficiency int     `json:"required_proficiency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	sk, err := svc.UpdateSkill(r.Context(), skillID, req.Description, req.Category, req.RiskLevel, req.RequiredTrustMin, req.RequiredProficiency)
	if err != nil {
		slog.Error("update skill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sk)
}

func DeleteSkill(w http.ResponseWriter, r *http.Request) {
	skillID := chi.URLParam(r, "skillID")
	if skillID == "" {
		http.Error(w, "skill_id required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	if err := svc.DeleteSkill(r.Context(), skillID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func AssignSkill(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		SkillID     string `json:"skill_id"`
		Proficiency int    `json:"proficiency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.SkillID == "" {
		http.Error(w, "skill_id is required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	as, err := svc.AssignSkill(r.Context(), agentID, req.SkillID, req.Proficiency)
	if err != nil {
		slog.Error("assign skill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(as)
}

func RemoveSkill(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	skillID := chi.URLParam(r, "skillID")
	if agentID == "" || skillID == "" {
		http.Error(w, "agent_id and skill_id required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	if err := svc.RemoveSkill(r.Context(), agentID, skillID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

func ListAgentSkills(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	skills, err := svc.ListAgentSkills(r.Context(), agentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if skills == nil {
		skills = []skill.AgentSkill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"skills":   skills,
	})
}

func VerifySkill(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	skillID := chi.URLParam(r, "skillID")
	if agentID == "" || skillID == "" {
		http.Error(w, "agent_id and skill_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		VerifiedBy string `json:"verified_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.VerifiedBy == "" {
		req.VerifiedBy = "admin"
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	as, err := svc.VerifySkill(r.Context(), agentID, skillID, req.VerifiedBy)
	if err != nil {
		slog.Error("verify skill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tracker := skill.NewSkillTrustTracker(querier)
	tracker.AdjustOnVerification(r.Context(), agentID, skillID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(as)
}

func EndorseSkill(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	skillID := chi.URLParam(r, "skillID")
	if agentID == "" || skillID == "" {
		http.Error(w, "agent_id and skill_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		EndorserType string `json:"endorser_type"`
		EndorserID   string `json:"endorser_id"`
		Comment      string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.EndorserType == "" {
		req.EndorserType = "human"
	}
	if req.EndorserID == "" {
		http.Error(w, "endorser_id is required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	e, err := svc.EndorseSkill(r.Context(), agentID, skillID, req.EndorserType, req.EndorserID, req.Comment)
	if err != nil {
		slog.Error("endorse skill failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tracker := skill.NewSkillTrustTracker(querier)
	tracker.AdjustOnEndorsement(r.Context(), agentID, skillID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(e)
}

func ListEndorsements(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}
	skillID := r.URL.Query().Get("skill_id")

	svc := skill.NewSkillServiceWithQuerier(querier)
	endorsements, err := svc.ListEndorsements(r.Context(), agentID, skillID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if endorsements == nil {
		endorsements = []skill.Endorsement{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":     agentID,
		"endorsements": endorsements,
	})
}

func GetSkillTrust(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	skillID := r.URL.Query().Get("skill_id")

	svc := skill.NewSkillServiceWithQuerier(querier)
	if skillID != "" {
		trust, err := svc.GetSkillTrust(r.Context(), agentID, skillID)
		if err != nil {
			http.Error(w, "skill trust score not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":   agentID,
			"skill_id":   skillID,
			"trust_score": trust,
		})
		return
	}

	scores, err := svc.GetAgentSkillTrust(r.Context(), agentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if scores == nil {
		scores = []skill.SkillTrustScore{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"scores":   scores,
	})
}

func CheckSkillAuthz(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	action := r.URL.Query().Get("action")
	if action == "" {
		http.Error(w, "action query parameter is required", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	allowed, reasons, trust, err := svc.CheckSkillAuthorization(r.Context(), agentID, action)
	if err != nil {
		slog.Error("skill authz check failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"action":   action,
		"allowed":  allowed,
		"reasons":  reasons,
		"trust":    trust,
	})
}

func FindMissingSkills(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		RequiredSkills []string `json:"required_skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := skill.NewSkillServiceWithQuerier(querier)
	missing, err := svc.FindMissingSkills(r.Context(), agentID, req.RequiredSkills)
	if err != nil {
		slog.Error("find missing skills failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if missing == nil {
		missing = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":       agentID,
		"missing_skills": missing,
	})
}

func SearchSkills(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	q := r.URL.Query().Get("q")

	svc := skill.NewSkillServiceWithQuerier(querier)
	skills, err := svc.ListSkills(r.Context(), category)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if skills == nil {
		skills = []skill.Skill{}
	}

	if q != "" {
		var filtered []skill.Skill
		for _, sk := range skills {
			if contains(sk.Name, q) || contains(sk.Description, q) || contains(sk.Category, q) {
				filtered = append(filtered, sk)
			}
		}
		skills = filtered
	}
	if skills == nil {
		skills = []skill.Skill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
		"count":  len(skills),
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func AdjustSkillTrust(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	skillID := chi.URLParam(r, "skillID")
	if agentID == "" || skillID == "" {
		http.Error(w, "agent_id and skill_id required", http.StatusBadRequest)
		return
	}

	var req struct {
		Delta  float64 `json:"delta"`
		Reason string  `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	delta := req.Delta
	if delta == 0 {
		deltaStr := r.URL.Query().Get("delta")
		if deltaStr != "" {
			if d, err := strconv.ParseFloat(deltaStr, 64); err == nil {
				delta = d
			}
		}
	}
	if delta == 0 {
		http.Error(w, "delta is required and must be non-zero", http.StatusBadRequest)
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "manual adjustment"
	}

	tracker := skill.NewSkillTrustTracker(querier)
	adj, err := tracker.AdjustSkillTrust(r.Context(), agentID, skillID, delta, reason)
	if err != nil {
		slog.Error("adjust skill trust failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(adj)
}