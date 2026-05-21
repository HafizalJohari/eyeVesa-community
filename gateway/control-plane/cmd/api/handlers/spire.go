package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
)

var spireService *identity.SpireService
var identityProvider identity.IdentityProvider

func SetSpireService(s *identity.SpireService) {
	spireService = s
}

func SetIdentityProvider(p identity.IdentityProvider) {
	identityProvider = p
}

func CreateTrustBundle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TrustDomain string `json:"trust_domain"`
		BundleData  string `json:"bundle_data"`
		BundleType  string `json:"bundle_type"`
		Source      string `json:"source"`
		EndpointURL string `json:"endpoint_url"`
		IsFederated bool   `json:"is_federated"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TrustDomain == "" || req.BundleData == "" {
		http.Error(w, "trust_domain and bundle_data are required", http.StatusBadRequest)
		return
	}

	if req.BundleType == "" {
		req.BundleType = "spiffe_x509"
	}
	if req.Source == "" {
		req.Source = "static"
	}

	bundle, err := spireService.CreateTrustBundle(r.Context(), req.TrustDomain, req.BundleData, req.BundleType, req.Source, req.EndpointURL, req.IsFederated)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bundle)
}

func GetTrustBundle(w http.ResponseWriter, r *http.Request) {
	trustDomain := chi.URLParam(r, "trustDomain")
	if trustDomain == "" {
		http.Error(w, "trust_domain required", http.StatusBadRequest)
		return
	}

	bundle, err := spireService.GetTrustBundle(r.Context(), trustDomain)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func ListTrustBundles(w http.ResponseWriter, r *http.Request) {
	federatedOnly := r.URL.Query().Get("federated") == "true"

	bundles, err := spireService.ListTrustBundles(r.Context(), federatedOnly)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if bundles == nil {
		bundles = []identity.TrustBundle{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bundles": bundles,
	})
}

func UpdateTrustBundle(w http.ResponseWriter, r *http.Request) {
	trustDomain := chi.URLParam(r, "trustDomain")

	var req struct {
		BundleData string `json:"bundle_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.BundleData == "" {
		http.Error(w, "bundle_data required", http.StatusBadRequest)
		return
	}

	bundle, err := spireService.UpdateTrustBundle(r.Context(), trustDomain, req.BundleData)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bundle)
}

func VerifyTrustBundle(w http.ResponseWriter, r *http.Request) {
	trustDomain := chi.URLParam(r, "trustDomain")

	if err := spireService.VerifyTrustBundle(r.Context(), trustDomain); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "verified",
		"trust_domain": trustDomain,
	})
}

func DeleteTrustBundle(w http.ResponseWriter, r *http.Request) {
	trustDomain := chi.URLParam(r, "trustDomain")

	if err := spireService.DeleteTrustBundle(r.Context(), trustDomain); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func FetchBundleFromEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EndpointURL string `json:"endpoint_url"`
		TrustDomain string `json:"trust_domain"`
		Save        bool   `json:"save"`
		IsFederated bool   `json:"is_federated"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.EndpointURL == "" {
		http.Error(w, "endpoint_url required", http.StatusBadRequest)
		return
	}

	bundleData, err := spireService.FetchBundleFromEndpoint(r.Context(), req.EndpointURL)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if req.Save && req.TrustDomain != "" {
		_, saveErr := spireService.CreateTrustBundle(r.Context(), req.TrustDomain, bundleData, "spiffe_x509", "web", req.EndpointURL, req.IsFederated)
		if saveErr != nil {
			slog.Error("failed to save fetched bundle", "error", saveErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trust_domain": req.TrustDomain,
		"bundle_data":  bundleData,
		"source":       "web",
	})
}

func RegisterWorkload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SpiffeID    string   `json:"spiffe_id"`
		AgentID     string   `json:"agent_id"`
		TrustDomain string   `json:"trust_domain"`
		Selectors   []string `json:"selectors"`
		ParentID    string   `json:"parent_id"`
		AutoRegister bool    `json:"auto_register"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SpiffeID == "" || req.AgentID == "" || req.TrustDomain == "" {
		http.Error(w, "spiffe_id, agent_id, and trust_domain are required", http.StatusBadRequest)
		return
	}

	if req.Selectors == nil {
		req.Selectors = []string{}
	}

	wl, err := spireService.RegisterWorkload(r.Context(), req.SpiffeID, req.AgentID, req.TrustDomain, req.Selectors, req.ParentID, req.AutoRegister)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wl)
}

func GetWorkload(w http.ResponseWriter, r *http.Request) {
	spiffeID := chi.URLParam(r, "spiffeID")

	wl, err := spireService.GetWorkload(r.Context(), spiffeID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wl)
}

func ListWorkloads(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")

	workloads, err := spireService.ListWorkloads(r.Context(), agentID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	if workloads == nil {
		workloads = []identity.WorkloadRegistration{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workloads": workloads,
	})
}

func AttestWorkload(w http.ResponseWriter, r *http.Request) {
	spiffeID := chi.URLParam(r, "spiffeID")

	wl, err := spireService.AttestWorkload(r.Context(), spiffeID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wl)
}

func DeleteWorkload(w http.ResponseWriter, r *http.Request) {
	spiffeID := chi.URLParam(r, "spiffeID")

	if err := spireService.DeleteWorkload(r.Context(), spiffeID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func GetSpireStatus(w http.ResponseWriter, r *http.Request) {
	if identityProvider == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"error":     "no identity provider configured",
		})
		return
	}

	status, err := spireService.GetStatus(r.Context(), identityProvider)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("handler error", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}