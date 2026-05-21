package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/a2a"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/delegation"
)

type delegationAdapter struct{}

func (d *delegationAdapter) Create(fromAgentID, toAgentID string, scope []string, duration time.Duration) (string, error) {
	if delegationTracker == nil {
		return "", nil
	}
	chain, err := delegationTracker.Delegate(context.Background(), delegation.DelegateRequest{
		ParentAgentID: fromAgentID,
		ChildAgentID:  toAgentID,
		Scope:         scope,
		MaxDepth:      1,
		Duration:      duration,
	})
	if err != nil {
		return "", err
	}
	return chain.DelegationID.String(), nil
}

var a2aService = a2a.NewService(&delegationAdapter{})

func ListA2AAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := querier.Query(r.Context(), `SELECT agent_id::text, name, owner, trust_score, status, capabilities FROM agents ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		http.Error(w, "failed to list agents", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cards := make([]a2a.AgentCard, 0)
	for rows.Next() {
		var agentID, name, owner, status string
		var trustScore float64
		var capabilities []string
		if err := rows.Scan(&agentID, &name, &owner, &trustScore, &status, &capabilities); err != nil {
			http.Error(w, "failed to scan agents", http.StatusInternalServerError)
			return
		}
		cards = append(cards, a2a.AgentCard{ID: agentID, Name: name, Owner: owner, Status: status, TrustScore: trustScore, Capabilities: capabilities})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"agents": cards, "count": len(cards)})
}

func CreateA2ATask(w http.ResponseWriter, r *http.Request) {
	var req a2a.TaskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	task, err := a2aService.CreateTask(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func GetA2ATask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}
	task, ok := a2aService.GetTask(taskID)
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}
