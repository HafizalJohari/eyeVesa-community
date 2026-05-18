package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/crypto"
)

var keyRotationService *crypto.KeyRotationService

func SetKeyRotationService(svc *crypto.KeyRotationService) {
	keyRotationService = svc
}

func RotateKey(w http.ResponseWriter, r *http.Request) {
	if keyRotationService == nil {
		http.Error(w, `{"error":"key rotation service not available"}`, http.StatusServiceUnavailable)
		return
	}

	newPub, err := keyRotationService.Rotate()
	if err != nil {
		http.Error(w, `{"error":"rotation failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "rotated",
		"new_public_key": crypto.EncodeBase64(newPub),
	})
}

func GetKeyRotationStatus(w http.ResponseWriter, r *http.Request) {
	if keyRotationService == nil {
		http.Error(w, `{"error":"key rotation service not available"}`, http.StatusServiceUnavailable)
		return
	}

	status := keyRotationService.GetRotationStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func ClearPreviousKey(w http.ResponseWriter, r *http.Request) {
	if keyRotationService == nil {
		http.Error(w, `{"error":"key rotation service not available"}`, http.StatusServiceUnavailable)
		return
	}

	keyRotationService.ClearPreviousKey()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "previous_key_cleared",
	})
}