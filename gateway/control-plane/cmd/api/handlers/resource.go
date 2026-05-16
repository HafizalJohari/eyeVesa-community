package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	CreatedAt  string    `json:"created_at"`
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

	resp := ResourceResponse{
		ResourceID: resourceID,
		Name:       req.Name,
		Type:       req.Type,
		Endpoint:   req.Endpoint,
		CreatedAt:  "now",
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "not_implemented",
		"resource_id": resourceIDStr,
	})
}