package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AgentRegistration struct {
	Name            string   `json:"name"`
	Owner           string   `json:"owner"`
	Capabilities    []string `json:"capabilities"`
	AllowedTools    []string `json:"allowed_tools"`
	MaxBudgetUSD    float64  `json:"max_budget_usd"`
	DelegationPolicy string  `json:"delegation_policy"`
	BehavioralTags  []string `json:"behavioral_tags"`
}

type AgentResponse struct {
	AgentID   uuid.UUID `json:"agent_id"`
	PublicKey string    `json:"public_key"`
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	CreatedAt string    `json:"created_at"`
}

func RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req AgentRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Owner == "" {
		http.Error(w, "name and owner are required", http.StatusBadRequest)
		return
	}

	agentID := uuid.New()

	resp := AgentResponse{
		AgentID:   agentID,
		PublicKey: "pending_key_generation",
		Name:      req.Name,
		Owner:     req.Owner,
		CreatedAt: "now",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetAgent(w http.ResponseWriter, r *http.Request) {
	agentIDStr := chi.URLParam(r, "agentID")
	if agentIDStr == "" {
		http.Error(w, "agent_id required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "not_implemented",
		"agent_id": agentIDStr,
	})
}