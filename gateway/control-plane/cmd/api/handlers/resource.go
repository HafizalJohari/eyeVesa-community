package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/audit"
)

type ResourceRegistration struct {
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Endpoint          string          `json:"endpoint"`
	AuthMethod        string          `json:"auth_method"`
	Capabilities      json.RawMessage `json:"capabilities"`
	RiskLevel         string          `json:"risk_level"`
	DataSensitivity   string          `json:"data_sensitivity"`
	RateLimitPerAgent int             `json:"rate_limit_per_agent"`
}

type ResourceResponse struct {
	ResourceID uuid.UUID `json:"resource_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Endpoint   string    `json:"endpoint"`
	CreatedAt  time.Time `json:"created_at"`
}

func RegisterResource(w http.ResponseWriter, r *http.Request) {
	var req ResourceRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Endpoint == "" {
		http.Error(w, "name and endpoint are required", http.StatusBadRequest)
		return
	}

	resourceID := uuid.New()

	authMethod := req.AuthMethod
	if authMethod == "" {
		authMethod = "mTLS+SVID"
	}
	riskLevel := req.RiskLevel
	if riskLevel == "" {
		riskLevel = "medium"
	}
	dataSensitivity := req.DataSensitivity
	if dataSensitivity == "" {
		dataSensitivity = "internal"
	}
	rateLimit := req.RateLimitPerAgent
	if rateLimit == 0 {
		rateLimit = 100
	}
	capabilities := req.Capabilities
	if capabilities == nil {
		capabilities = json.RawMessage(`{}`)
	}

	var createdAt time.Time
	err := querier.QueryRow(r.Context(),
		`INSERT INTO resources (resource_id, name, resource_type, endpoint, auth_method, capabilities, risk_level, data_sensitivity, rate_limit_per_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING created_at`,
		resourceID, req.Name, req.Type, req.Endpoint, authMethod,
		capabilities, riskLevel, dataSensitivity, rateLimit,
	).Scan(&createdAt)

	if err != nil {
		log.Printf("RegisterResource: database insert failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	auditEntry := audit.AuditEntry{
		ResourceID:  resourceID.String(),
		Action:      "resource.register",
		Method:      "POST",
		Status:      "success",
		TrustBefore: 1.0,
		TrustAfter:  1.0,
	}
	auditLogger.Log(r.Context(), auditEntry, gatewayPrivateKey)

	resp := ResourceResponse{
		ResourceID: resourceID,
		Name:       req.Name,
		Type:       req.Type,
		Endpoint:   req.Endpoint,
		CreatedAt:  createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetResource(w http.ResponseWriter, r *http.Request) {
	resourceIDStr := chi.URLParam(r, "resourceID")
	if resourceIDStr == "" {
		http.Error(w, "resource_id required", http.StatusBadRequest)
		return
	}

	var name, resourceType, endpoint, status string
	var riskLevel string
	err := querier.QueryRow(r.Context(),
		`SELECT name, resource_type, endpoint, risk_level, status FROM resources WHERE resource_id = $1`,
		resourceIDStr,
	).Scan(&name, &resourceType, &endpoint, &riskLevel, &status)

	if err != nil {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resource_id":   resourceIDStr,
		"name":          name,
		"resource_type": resourceType,
		"endpoint":      endpoint,
		"risk_level":    riskLevel,
		"status":        status,
	})
}

func ListResources(w http.ResponseWriter, r *http.Request) {
	rows, err := querier.Query(r.Context(),
		`SELECT resource_id, name, resource_type, endpoint, risk_level, status FROM resources ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	resources := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, name, resourceType, endpoint, riskLevel, status string
		if err := rows.Scan(&id, &name, &resourceType, &endpoint, &riskLevel, &status); err != nil {
			continue
		}
		resources = append(resources, map[string]interface{}{
			"resource_id":   id,
			"name":          name,
			"resource_type": resourceType,
			"endpoint":      endpoint,
			"risk_level":    riskLevel,
			"status":        status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"resources": resources,
	})
}