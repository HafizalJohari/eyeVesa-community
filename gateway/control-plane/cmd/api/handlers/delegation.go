package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/delegation"
)

type DelegateRequest struct {
	ParentAgentID string   `json:"parent_agent_id"`
	ChildAgentID  string   `json:"child_agent_id"`
	Scope         []string `json:"scope"`
	MaxDepth      int      `json:"max_depth"`
	Duration      string   `json:"duration"`
}

type DelegateResponse struct {
	DelegationID string    `json:"delegation_id"`
	ParentAgentID string   `json:"parent_agent_id"`
	ChildAgentID  string   `json:"child_agent_id"`
	Scope         []string `json:"scope"`
	MaxDepth      int      `json:"max_depth"`
	ExpiresAt     string   `json:"expires_at"`
	SpiffeID      string   `json:"spiffe_id"`
}

var delegationTracker *delegation.DelegationTracker

func SetDelegationTracker(dt *delegation.DelegationTracker) {
	delegationTracker = dt
}

func DelegateAgent(w http.ResponseWriter, r *http.Request) {
	var req DelegateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ParentAgentID == "" || req.ChildAgentID == "" {
		http.Error(w, "parent_agent_id and child_agent_id are required", http.StatusBadRequest)
		return
	}

	duration := 1 * time.Hour
	if req.Duration != "" {
		if d, err := time.ParseDuration(req.Duration); err == nil {
			duration = d
		}
	}

	maxDepth := req.MaxDepth
	if maxDepth == 0 {
		maxDepth = 1
	}

	chain, err := delegationTracker.Delegate(r.Context(), delegation.DelegateRequest{
		ParentAgentID: req.ParentAgentID,
		ChildAgentID:  req.ChildAgentID,
		Scope:         req.Scope,
		MaxDepth:      maxDepth,
		Duration:      duration,
	})
	if err != nil {
		slog.Error("delegation failed", "error", err)
		http.Error(w, "internal error", http.StatusBadRequest)
		return
	}

	spiffeID := ""
	if chain.SVID != nil {
		spiffeID = chain.SVID.SpiffeID
	}

	resp := DelegateResponse{
		DelegationID:  chain.DelegationID.String(),
		ParentAgentID: chain.ParentAgentID.String(),
		ChildAgentID:  chain.ChildAgentID.String(),
		Scope:         chain.Scope,
		MaxDepth:      chain.MaxDepth,
		ExpiresAt:     chain.ExpiresAt.Format(time.RFC3339),
		SpiffeID:      spiffeID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetDelegationChain(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	chains, err := delegationTracker.GetDelegationChain(r.Context(), agentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0)
	for _, c := range chains {
		result = append(result, map[string]interface{}{
			"delegation_id":  c.DelegationID.String(),
			"parent_agent_id": c.ParentAgentID.String(),
			"child_agent_id":  c.ChildAgentID.String(),
			"scope":          c.Scope,
			"max_depth":      c.MaxDepth,
			"expires_at":     c.ExpiresAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegations": result,
	})
}

func ValidateDelegation(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent")
	childID := r.URL.Query().Get("child")
	if parentID == "" || childID == "" {
		http.Error(w, "parent and child query params required", http.StatusBadRequest)
		return
	}

	valid, err := delegationTracker.ValidateDelegation(r.Context(), parentID, childID)
	if err != nil {
		http.Error(w, "validation error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"parent_agent_id": parentID,
		"child_agent_id":  childID,
		"valid":           valid,
	})
}

func RevokeDelegation(w http.ResponseWriter, r *http.Request) {
	delegationID := chi.URLParam(r, "delegationID")
	if delegationID == "" {
		http.Error(w, "delegation_id required", http.StatusBadRequest)
		return
	}

	if err := delegationTracker.Revoke(r.Context(), delegationID); err != nil {
		http.Error(w, "revoke error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"delegation_id": delegationID,
		"status":        "revoked",
	})
}