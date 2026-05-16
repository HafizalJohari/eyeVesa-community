package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ToolHandler func(params json.RawMessage) (interface{}, error)
type ResourceHandler func(uri string) (interface{}, error)
type PromptHandler func(args map[string]string) (string, error)

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

type PromptDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ResourceAdapter struct {
	name        string
	endpoint    string
	resourceID  uuid.UUID
	tools       map[string]ToolHandler
	toolDefs    map[string]ToolDefinition
	resources   map[string]ResourceHandler
	resDefs     map[string]ResourceDefinition
	prompts     map[string]PromptHandler
	promptDefs  map[string]PromptDefinition
	mu          sync.RWMutex
	registered  bool
	httpClient   *http.Client
}

func New(name, endpoint string) *ResourceAdapter {
	return &ResourceAdapter{
		name:       name,
		endpoint:   endpoint,
		resourceID: uuid.Nil,
		tools:      make(map[string]ToolHandler),
		toolDefs:   make(map[string]ToolDefinition),
		resources:  make(map[string]ResourceHandler),
		resDefs:    make(map[string]ResourceDefinition),
		prompts:    make(map[string]PromptHandler),
		promptDefs: make(map[string]PromptDefinition),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *ResourceAdapter) RegisterTool(name string, description string, inputSchema map[string]interface{}, handler ToolHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools[name] = handler
	a.toolDefs[name] = ToolDefinition{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

func (a *ResourceAdapter) RegisterResource(uri, name, description, mimeType string, handler ResourceHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resources[uri] = handler
	a.resDefs[uri] = ResourceDefinition{
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    mimeType,
	}
}

func (a *ResourceAdapter) RegisterPrompt(name, description string, handler PromptHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prompts[name] = handler
	a.promptDefs[name] = PromptDefinition{
		Name:        name,
		Description: description,
	}
}

func (a *ResourceAdapter) RegisterWithGateway(ctx context.Context) error {
	gatewayURL := fmt.Sprintf("http://%s/v1/resources/register", a.endpoint)

	payload := map[string]interface{}{
		"name":             a.name,
		"type":             "mcp-server",
		"endpoint":         fmt.Sprintf("http://localhost:8443/mcp"),
		"auth_method":      "agentid",
		"risk_level":       "medium",
		"data_sensitivity": "internal",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal registration: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register with gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s: %s", resp.Status, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if id, ok := result["resource_id"].(string); ok {
		if parsed, err := uuid.Parse(id); err == nil {
			a.resourceID = parsed
		}
	}

	a.registered = true
	log.Printf("Registered with gateway as %s (resource_id: %s)", a.name, a.resourceID)
	return nil
}

func (a *ResourceAdapter) Run(ctx context.Context) error {
	if err := a.RegisterWithGateway(ctx); err != nil {
		log.Printf("WARN: Failed to register with gateway: %v (continuing anyway)", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", a.handleMCP)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := ":8443"
	log.Printf("Resource adapter '%s' listening on %s", a.name, addr)

	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return server.ListenAndServe()
}

func (a *ResourceAdapter) handleMCP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JsonRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      nil,
			"error":   map[string]interface{}{"code": -32700, "message": "Parse error"},
		})
		return
	}

	var result interface{}
	var rpcErr interface{}

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
				"name":    a.name,
				"version": "0.1.0",
			},
		}

	case "tools/list":
		a.mu.RLock()
		tools := make([]ToolDefinition, 0, len(a.toolDefs))
		for _, def := range a.toolDefs {
			tools = append(tools, def)
		}
		a.mu.RUnlock()
		result = map[string]interface{}{"tools": tools}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Invalid params: %v", err),
			}
			break
		}
		a.mu.RLock()
		handler, ok := a.tools[params.Name]
		a.mu.RUnlock()
		if !ok {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Unknown tool: %s", params.Name),
			}
			break
		}
		argsJSON, _ := json.Marshal(params.Arguments)
		handlerResult, err := handler(argsJSON)
		if err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32603,
				"message": fmt.Sprintf("Tool error: %v", err),
			}
			break
		}
		result = map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("%v", handlerResult)},
			},
		}

	case "resources/list":
		a.mu.RLock()
		resources := make([]ResourceDefinition, 0, len(a.resDefs))
		for _, def := range a.resDefs {
			resources = append(resources, def)
		}
		a.mu.RUnlock()
		result = map[string]interface{}{"resources": resources}

	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Invalid params: %v", err),
			}
			break
		}
		a.mu.RLock()
		handler, ok := a.resources[params.URI]
		a.mu.RUnlock()
		if !ok {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Unknown resource: %s", params.URI),
			}
			break
		}
		handlerResult, err := handler(params.URI)
		if err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32603,
				"message": fmt.Sprintf("Resource error: %v", err),
			}
			break
		}
		result = map[string]interface{}{
			"contents": []map[string]interface{}{
				{"uri": params.URI, "mimeType": "application/json", "text": fmt.Sprintf("%v", handlerResult)},
			},
		}

	case "prompts/list":
		a.mu.RLock()
		prompts := make([]PromptDefinition, 0, len(a.promptDefs))
		for _, def := range a.promptDefs {
			prompts = append(prompts, def)
		}
		a.mu.RUnlock()
		result = map[string]interface{}{"prompts": prompts}

	case "prompts/get":
		var params struct {
			Name string            `json:"name"`
			Args map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Invalid params: %v", err),
			}
			break
		}
		a.mu.RLock()
		handler, ok := a.prompts[params.Name]
		a.mu.RUnlock()
		if !ok {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": fmt.Sprintf("Unknown prompt: %s", params.Name),
			}
			break
		}
		promptResult, err := handler(params.Args)
		if err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32603,
				"message": fmt.Sprintf("Prompt error: %v", err),
			}
			break
		}
		result = map[string]interface{}{
			"messages": []map[string]interface{}{
				{"role": "user", "content": map[string]interface{}{"type": "text", "text": promptResult}},
			},
		}

	default:
		rpcErr = map[string]interface{}{
			"code":    -32601,
			"message": fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	json.NewEncoder(w).Encode(resp)
}