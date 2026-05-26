package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/identity"
)

var federationService *identity.FederationService

func SetFederationService(fs *identity.FederationService) {
	federationService = fs
}

func RegisterFederationPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		PublicKey   string `json:"public_key"`
		Endpoint    string `json:"endpoint"`
		TrustDomain string `json:"trust_domain"`
		PeerType    string `json:"peer_type"`
		InviteToken string `json:"invite_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.PublicKey == "" || req.Endpoint == "" {
		http.Error(w, "name, public_key, and endpoint are required", http.StatusBadRequest)
		return
	}

	if req.TrustDomain == "" {
		req.TrustDomain = req.Name
	}
	if req.PeerType == "" {
		req.PeerType = "community"
	}

	peer, err := federationService.RegisterPeer(r.Context(), req.Name, req.PublicKey, req.Endpoint, req.TrustDomain, req.PeerType, req.InviteToken, false)
	if err != nil {
		slog.Error("register federation peer failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(peer)
}

func RegisterFederationPeerAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		PublicKey   string `json:"public_key"`
		Endpoint    string `json:"endpoint"`
		TrustDomain string `json:"trust_domain"`
		PeerType    string `json:"peer_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.PublicKey == "" || req.Endpoint == "" {
		http.Error(w, "name, public_key, and endpoint are required", http.StatusBadRequest)
		return
	}
	if req.TrustDomain == "" {
		req.TrustDomain = req.Name
	}
	if req.PeerType == "" {
		req.PeerType = "community"
	}
	peer, err := federationService.RegisterPeer(r.Context(), req.Name, req.PublicKey, req.Endpoint, req.TrustDomain, req.PeerType, "", true)
	if err != nil {
		slog.Error("admin register federation peer failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(peer)
}

func CreateFederationInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Endpoint    string `json:"endpoint"`
		TrustDomain string `json:"trust_domain"`
		PeerType    string `json:"peer_type"`
		TTLHours    int    `json:"ttl_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Endpoint == "" {
		http.Error(w, "name and endpoint are required", http.StatusBadRequest)
		return
	}
	if req.PeerType == "" {
		req.PeerType = "community"
	}
	ttl := time.Duration(req.TTLHours) * time.Hour
	invite, err := federationService.CreatePeerInvite(r.Context(), req.Name, req.Endpoint, req.TrustDomain, req.PeerType, ttl)
	if err != nil {
		slog.Error("create federation invite failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(invite)
}

func GetFederationPeer(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	if gatewayID == "" {
		gatewayID = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	}

	peer, err := federationService.GetPeer(r.Context(), gatewayID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "peer not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peer)
}

func ListFederationPeers(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	peerType := r.URL.Query().Get("peer_type")

	peers, err := federationService.ListPeers(r.Context(), status, peerType)
	if err != nil {
		slog.Error("list federation peers failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal_error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

func SyncFederatedAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Passport     identity.AgentPassport `json:"passport"`
		Name         string                 `json:"name"`
		Owner        string                 `json:"owner"`
		TrustScore   float64                `json:"trust_score"`
		Capabilities []string               `json:"capabilities"`
		AllowedTools []string               `json:"allowed_tools"`
		Description  string                 `json:"description"`
		Tags         []string               `json:"tags"`
		Scope        string                 `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passport.AgentID == "" || req.Passport.GatewayID == "" || req.Passport.GatewaySig == "" {
		http.Error(w, "passport with agent_id, gateway_id, and gateway_signature are required", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Owner == "" {
		http.Error(w, "name and owner are required", http.StatusBadRequest)
		return
	}

	if req.TrustScore <= 0 {
		req.TrustScore = 1.0
	}
	if req.Scope == "" {
		req.Scope = "international"
	}

	agent, err := federationService.SyncAgent(
		r.Context(), req.Passport, req.Name, req.Owner, req.TrustScore,
		req.Capabilities, req.AllowedTools, req.Description, req.Tags, req.Scope,
	)
	if err != nil {
		slog.Error("sync federated agent failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(agent)
}

func AuthorizeFederatedInvokeHandler(w http.ResponseWriter, r *http.Request) {
	var req identity.FederatedInvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	decision, err := federationService.AuthorizeFederatedInvoke(r.Context(), req)
	if err != nil {
		slog.Error("federated invoke authorization failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}

func FederatedHeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID   string          `json:"agent_id"`
		GatewayID string          `json:"gateway_id"`
		Status    string          `json:"status"`
		Metadata  json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.GatewayID == "" {
		http.Error(w, "agent_id and gateway_id are required", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{"online": true, "offline": true, "busy": true, "idle": true}
	if !validStatuses[req.Status] {
		req.Status = "online"
	}

	err := federationService.FederatedHeartbeat(r.Context(), req.AgentID, req.GatewayID, req.Status, req.Metadata)
	if err != nil {
		slog.Error("federated heartbeat failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id":   req.AgentID,
		"gateway_id": req.GatewayID,
		"status":     req.Status,
		"ok":         true,
	})
}

func SearchFederatedAgentsHandler(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	tag := r.URL.Query().Get("tag")
	owner := r.URL.Query().Get("owner")
	scope := r.URL.Query().Get("scope")
	minTrust := 0.0
	if v := r.URL.Query().Get("min_trust"); v != "" {
		if f, err := parseFloat(v); err == nil {
			minTrust = f
		}
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if i, err := parseInt(v); err == nil && i > 0 {
			limit = i
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if i, err := parseInt(v); err == nil && i >= 0 {
			offset = i
		}
	}

	agents, err := federationService.SearchFederatedAgents(r.Context(), status, tag, owner, minTrust, scope, limit, offset)
	if err != nil {
		slog.Error("federated agent search failed", "error", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
		"limit":  limit,
		"offset": offset,
	})
}

func ListFederatedOnlineHandler(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	agents, err := federationService.ListFederatedOnline(r.Context(), scope)
	if err != nil {
		slog.Error("list federated online failed", "error", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

func GetFederatedAgentHandler(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if agentID == "" {
		agentID = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	}

	agent, err := federationService.GetFederatedAgent(r.Context(), agentID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "agent not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func SuspendFederationPeerHandler(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	if gatewayID == "" {
		http.Error(w, "gateway_id required", http.StatusBadRequest)
		return
	}

	err := federationService.SuspendPeer(r.Context(), gatewayID)
	if err != nil {
		slog.Error("suspend federation peer failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"gateway_id": gatewayID,
		"status":     "suspended",
		"ok":         true,
	})
}

func FederationHealthHandler(w http.ResponseWriter, r *http.Request) {
	peers, _ := federationService.ListPeers(r.Context(), "active", "")
	online, _ := federationService.ListFederatedOnline(r.Context(), "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                  "healthy",
		"active_gateways":         len(peers),
		"online_federated_agents": len(online),
	})
}

func SyncFederatedMerchantTrust(w http.ResponseWriter, r *http.Request) {
	var req identity.FederatedMerchantTrustSync
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.MerchantID == "" || req.GatewayID == "" {
		http.Error(w, "merchant_id and gateway_id are required", http.StatusBadRequest)
		return
	}
	if err := federationService.SyncMerchantTrust(r.Context(), req); err != nil {
		slog.Error("sync federated merchant trust failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
