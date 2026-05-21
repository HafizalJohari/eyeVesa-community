package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
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

type MCPServer struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

var mcpHTTPClient = &http.Client{Timeout: 10 * time.Second}

func HandleMCP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeMCPError(w, nil, -32700, "Parse error")
		return
	}

	var req JsonRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeMCPError(w, nil, -32700, "Parse error")
		return
	}

	agentID := r.Header.Get("X-Agent-Id")

	switch req.Method {
	case "initialize":
		handleMCPInitialize(w, r, &req)
	case "tools/list", "resources/list", "prompts/list":
		handleMCPList(w, r, &req, bodyBytes)
	case "tools/call", "resources/read", "prompts/get":
		handleMCPCall(w, r, &req, bodyBytes, agentID)
	default:
		writeMCPError(w, req.ID, -32601, "Method not found: "+req.Method)
	}
}

func handleMCPInitialize(w http.ResponseWriter, r *http.Request, req *JsonRPCRequest) {
	writeMCPResult(w, req.ID, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{"listChanged": true},
			"resources": map[string]interface{}{"subscribe": true},
			"prompts":   map[string]interface{}{"listChanged": true},
		},
		"serverInfo": map[string]string{
			"name":    "eyevesa-gateway",
			"version": "0.2.0",
		},
		"kya": map[string]interface{}{
			"enabled":       true,
			"trustRequired": true,
		},
	})
}

func getMCPServers(ctx context.Context) []MCPServer {
	rows, err := querier.Query(ctx, `
		SELECT name, endpoint FROM resources
		WHERE resource_type = 'mcp-server' AND status = 'active'
		ORDER BY name
	`)
	if err != nil {
		slog.Error("get MCP servers failed", "error", err)
		return nil
	}
	defer rows.Close()

	var servers []MCPServer
	for rows.Next() {
		var s MCPServer
		if err := rows.Scan(&s.Name, &s.Endpoint); err != nil {
			continue
		}
		servers = append(servers, s)
	}
	return servers
}

func proxyToMCPServer(endpoint string, body []byte) (map[string]interface{}, error) {
	resp, err := mcpHTTPClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("MCP server unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("MCP server response invalid: %w", err)
	}
	return result, nil
}

func handleMCPList(w http.ResponseWriter, r *http.Request, req *JsonRPCRequest, rawBody []byte) {
	servers := getMCPServers(r.Context())
	if len(servers) == 0 {
		writeMCPResult(w, req.ID, map[string]interface{}{
			"tools":     []interface{}{},
			"resources": []interface{}{},
			"prompts":   []interface{}{},
		})
		return
	}

	// First initialize connection to each MCP server
	initBody, _ := json.Marshal(JsonRPCRequest{
		JsonRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})
	for _, s := range servers {
		proxyToMCPServer(s.Endpoint, initBody)
	}

	// Forward list request to each server and aggregate
	allTools := []interface{}{}
	allResources := []interface{}{}
	allPrompts := []interface{}{}

	for _, s := range servers {
		respData, err := proxyToMCPServer(s.Endpoint, rawBody)
		if err != nil {
			slog.Warn("MCP server list failed", "server", s.Name, "error", err)
			continue
		}
		if result, ok := respData["result"].(map[string]interface{}); ok {
			if tools, ok := result["tools"].([]interface{}); ok {
				for _, t := range tools {
					if tool, ok := t.(map[string]interface{}); ok {
						tool["_server"] = s.Name
						allTools = append(allTools, tool)
					}
				}
			}
			if resources, ok := result["resources"].([]interface{}); ok {
				for _, res := range resources {
					if rsrc, ok := res.(map[string]interface{}); ok {
						rsrc["_server"] = s.Name
						allResources = append(allResources, rsrc)
					}
				}
			}
			if prompts, ok := result["prompts"].([]interface{}); ok {
				for _, p := range prompts {
					if prompt, ok := p.(map[string]interface{}); ok {
						prompt["_server"] = s.Name
						allPrompts = append(allPrompts, prompt)
					}
				}
			}
		}
	}

	writeMCPResult(w, req.ID, map[string]interface{}{
		"tools":     allTools,
		"resources": allResources,
		"prompts":   allPrompts,
	})
}

func handleMCPCall(w http.ResponseWriter, r *http.Request, req *JsonRPCRequest, rawBody []byte, agentID string) {
	if agentID == "" {
		writeMCPError(w, req.ID, -32000, "KYA: Agent identity required for tool execution")
		return
	}

	// Verify agent's trust score
	var trustScore float64
	err := querier.QueryRow(r.Context(),
		`SELECT trust_score FROM agents WHERE agent_id = $1 AND status = 'active'`, agentID,
	).Scan(&trustScore)
	if err != nil {
		writeMCPError(w, req.ID, -32001, "KYA: Agent not found or inactive")
		return
	}
	if trustScore < 0.5 {
		writeMCPError(w, req.ID, -32002, fmt.Sprintf("KYA: Agent trust score %.2f below minimum 0.5", trustScore))
		return
	}

	servers := getMCPServers(r.Context())
	if len(servers) == 0 {
		writeMCPResult(w, req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "No MCP servers available"},
			},
		})
		return
	}

	// Try each server until one handles the request
	for _, s := range servers {
		respData, err := proxyToMCPServer(s.Endpoint, rawBody)
		if err != nil {
			slog.Warn("MCP server call failed", "server", s.Name, "error", err)
			continue
		}
		if respData["error"] != nil {
			continue
		}
		if result, ok := respData["result"].(map[string]interface{}); ok {
			result["_server"] = s.Name
			writeMCPResult(w, req.ID, result)
			return
		}
	}

	writeMCPError(w, req.ID, -32003, "No MCP server could handle the request")
}

func writeMCPResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JsonRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeMCPError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JsonRPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error:   &RpcError{Code: code, Message: message},
	})
}