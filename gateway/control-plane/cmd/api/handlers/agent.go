package handlers

import (
	"crypto/ed25519"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/policy"
)

type AgentRegistration struct {
	Name             string   `json:"name"`
	Owner            string   `json:"owner"`
	Capabilities     []string `json:"capabilities"`
	AllowedTools     []string `json:"allowed_tools"`
	MaxBudgetUSD     float64  `json:"max_budget_usd"`
	DelegationPolicy string   `json:"delegation_policy"`
	BehavioralTags   []string `json:"behavioral_tags"`
}

type AgentResponse struct {
	AgentID    uuid.UUID `json:"agent_id"`
	PublicKey  string    `json:"public_key"`
	Name       string    `json:"name"`
	Owner      string    `json:"owner"`
	Status     string    `json:"status"`
	TrustScore float64   `json:"trust_score"`
	CreatedAt  time.Time `json:"created_at"`
}

var db *database.DB
var querier database.Querier
var auditLogger *audit.AuditLogger
var gatewayPrivateKey ed25519.PrivateKey
var globalPolicyEngine *policy.PolicyEngine

func SetDB(d *database.DB) {
	db = d
	querier = &database.PoolQuerier{Pool: d.Pool}
}

func SetQuerier(q database.Querier) {
	querier = q
}

func SetAuditLogger(a *audit.AuditLogger) {
	auditLogger = a
}

func SetGatewayKeys(privKey ed25519.PrivateKey) {
	gatewayPrivateKey = privKey
}

func SetPolicyEngine(pe *policy.PolicyEngine) {
	globalPolicyEngine = pe
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

	keypair, err := crypto.GenerateAgentKeypair()
	if err != nil {
		http.Error(w, "failed to generate keypair", http.StatusInternalServerError)
		return
	}

	agentID := uuid.New()
	capabilities := req.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	allowedTools := req.AllowedTools
	if allowedTools == nil {
		allowedTools = []string{}
	}
	behavioralTags := req.BehavioralTags
	if behavioralTags == nil {
		behavioralTags = []string{}
	}
	delegationPolicy := req.DelegationPolicy
	if delegationPolicy == "" {
		delegationPolicy = "no_chain"
	}

	var createdAt time.Time
	err = querier.QueryRow(r.Context(),
		`INSERT INTO agents (agent_id, name, owner, public_key, capabilities, allowed_tools, max_budget_usd, delegation_policy, behavioral_tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING created_at`,
		agentID, req.Name, req.Owner, keypair.PublicKey, capabilities, allowedTools,
		req.MaxBudgetUSD, delegationPolicy, behavioralTags,
	).Scan(&createdAt)

	if err != nil {
		log.Printf("RegisterAgent: database insert failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if auditLogger != nil && gatewayPrivateKey != nil {
		auditEntry := audit.AuditEntry{
			AgentID:     agentID.String(),
			Action:      "agent.register",
			Method:      "HTTP",
			Status:      "success",
			TrustBefore: 1.0,
			TrustAfter:  1.0,
		}
		auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)
	}

	resp := AgentResponse{
		AgentID:    agentID,
		PublicKey:  crypto.EncodeBase64(keypair.PublicKey),
		Name:       req.Name,
		Owner:      req.Owner,
		Status:     "active",
		TrustScore: 1.0,
		CreatedAt:  createdAt,
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

	var name, owner, agentStatus string
	var trustScore float64
	var capabilities, allowedTools []string
	err := querier.QueryRow(r.Context(),
		`SELECT name, owner, trust_score, status, capabilities, allowed_tools FROM agents WHERE agent_id = $1`,
		agentIDStr,
	).Scan(&name, &owner, &trustScore, &agentStatus, &capabilities, &allowedTools)

	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":      agentIDStr,
		"name":          name,
		"owner":         owner,
		"trust_score":   trustScore,
		"status":        agentStatus,
		"capabilities":  capabilities,
		"allowed_tools": allowedTools,
	})
}

func ListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := querier.Query(r.Context(),
		`SELECT agent_id, name, owner, trust_score, status FROM agents ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, name, owner, agentStatus string
		var trustScore float64
		if err := rows.Scan(&id, &name, &owner, &trustScore, &agentStatus); err != nil {
			continue
		}
		agents = append(agents, map[string]interface{}{
			"agent_id":    id,
			"name":        name,
			"owner":       owner,
			"trust_score": trustScore,
			"status":      agentStatus,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
	})
}