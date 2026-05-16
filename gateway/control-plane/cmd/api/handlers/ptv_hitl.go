package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/hitl"
	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/ptv"
)

var hitlService *hitl.HITLService
var ptvService *ptv.PTVService

func SetHITLService(s *hitl.HITLService) {
	hitlService = s
}

func SetPTVService(s *ptv.PTVService) {
	ptvService = s
}

func RequestApproval(w http.ResponseWriter, r *http.Request) {
	var req hitl.ApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Action == "" {
		http.Error(w, "agent_id and action are required", http.StatusBadRequest)
		return
	}

	resp, err := hitlService.RequestApproval(r.Context(), req)
	if err != nil {
		log.Printf("RequestApproval: failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetApprovalStatus(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalID")
	if approvalID == "" {
		http.Error(w, "approval_id required", http.StatusBadRequest)
		return
	}

	status, err := hitlService.GetStatus(r.Context(), approvalID)
	if err != nil {
		http.Error(w, "approval not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"approval_id": approvalID,
		"status":      status,
	})
}

func DecideApproval(w http.ResponseWriter, r *http.Request) {
	var decision hitl.ApprovalDecision
	if err := json.NewDecoder(r.Body).Decode(&decision); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	approverMethod := decision.ApproverMethod
	if approverMethod == "" {
		approverMethod = "manual"
	}

	err := hitlService.Approve(r.Context(), hitl.ApprovalDecision{
		ApprovalID:     decision.ApprovalID,
		Approved:       decision.Approved,
		ApproverMethod: approverMethod,
	})
	if err != nil {
		log.Printf("DecideApproval: failed: %v", err)
		http.Error(w, "internal error", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approval_id":     decision.ApprovalID,
		"approved":       decision.Approved,
		"approver_method": approverMethod,
		"status":         map[bool]string{true: "approved", false: "rejected"}[decision.Approved],
	})
}

func ListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")

	results, err := hitlService.ListPending(r.Context(), agentID)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approvals": results,
	})
}

func AttestIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID         string `json:"agent_id"`
		Platform        string `json:"platform"`
		FirmwareVersion string `json:"firmware_version"`
		TPMPublicKeyB64 string `json:"tpm_public_key_base64"`
		RuntimeHashB64  string `json:"runtime_hash_base64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tpmPublicKey := []byte(req.TPMPublicKeyB64)
	if len(tpmPublicKey) == 0 {
		tpmPublicKey = []byte("simulated-tpm-key-" + req.AgentID)
	}
	runtimeHash := []byte(req.RuntimeHashB64)
	if len(runtimeHash) == 0 {
		runtimeHash = []byte("simulated-hash-" + req.Platform)
	}

	proof, err := ptvService.Prove(r.Context(), req.AgentID, req.Platform, req.FirmwareVersion, tpmPublicKey, runtimeHash, []byte("nonce-1234"))
	if err != nil {
		log.Printf("AttestIdentity: PTV prove failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proof)
}

func BindIdentity(w http.ResponseWriter, r *http.Request) {
	var proof ptv.AttestationProof
	if err := json.NewDecoder(r.Body).Decode(&proof); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	binding, err := ptvService.Transform(r.Context(), &proof, proof.Attestation.TPMPublicKey)
	if err != nil {
		log.Printf("BindIdentity: PTV transform failed: %v", err)
		http.Error(w, "internal error", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(binding)
}

func VerifyIdentity(w http.ResponseWriter, r *http.Request) {
	bindingID := chi.URLParam(r, "bindingID")
	if bindingID == "" {
		http.Error(w, "binding_id required", http.StatusBadRequest)
		return
	}

	result, err := ptvService.Verify(r.Context(), bindingID)
	if err != nil {
		log.Printf("VerifyIdentity: verification failed: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}