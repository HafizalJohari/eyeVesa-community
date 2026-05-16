package handlers

import (
	"encoding/json"
	"net/http"
)

type JsonRPCRequest struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JsonRPCResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RpcError   `json:"error,omitempty"`
}

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func HandleMCP(w http.ResponseWriter, r *http.Request) {
	var req JsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(JsonRPCResponse{
			JsonRPC: "2.0",
			ID:      nil,
			Error:   &RpcError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	var result interface{}

	switch req.Method {
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{"listChanged": true},
				"resources": map[string]interface{}{"subscribe": true},
				"prompts":   map[string]interface{}{"listChanged": true},
			},
			"serverInfo": map[string]string{
				"name":    "agentid-gateway",
				"version": "0.1.0",
			},
		}
	case "tools/list":
		result = map[string]interface{}{"tools": []interface{}{}}
	case "resources/list":
		result = map[string]interface{}{"resources": []interface{}{}}
	case "prompts/list":
		result = map[string]interface{}{"prompts": []interface{}{}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(JsonRPCResponse{
			JsonRPC: "2.0",
			ID:      req.ID,
			Error:   &RpcError{Code: -32601, Message: "Method not found: " + req.Method},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JsonRPCResponse{
		JsonRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}