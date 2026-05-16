package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type ResourceAdapter struct {
	name       string
	endpoint   string
	resourceID uuid.UUID
	tools      map[string]ToolHandler
	resources  map[string]ResourceHandler
	prompts    map[string]PromptHandler
	mu         sync.RWMutex
}

type ToolHandler func(params json.RawMessage) (interface{}, error)
type ResourceHandler func(uri string) (interface{}, error)
type PromptHandler func(args map[string]string) (string, error)

func New(name, endpoint string) *ResourceAdapter {
	return &ResourceAdapter{
		name:      name,
		endpoint:  endpoint,
		resourceID: uuid.Nil,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		prompts:   make(map[string]PromptHandler),
	}
}

func (a *ResourceAdapter) RegisterTool(name string, handler ToolHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools[name] = handler
}

func (a *ResourceAdapter) RegisterResource(uri string, handler ResourceHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resources[uri] = handler
}

func (a *ResourceAdapter) RegisterPrompt(name string, handler PromptHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prompts[name] = handler
}

func (a *ResourceAdapter) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", a.handleMCP)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := ":8443"
	fmt.Printf("Resource adapter listening on %s\n", addr)

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
			"error":   map[string]interface{}{"code": -32700, "message": "Parse error"},
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
				"name":    a.name,
				"version": "0.1.0",
			},
		}
	case "tools/list":
		a.mu.RLock()
		tools := make([]map[string]string, 0, len(a.tools))
		for name := range a.tools {
			tools = append(tools, map[string]string{"name": name})
		}
		a.mu.RUnlock()
		result = map[string]interface{}{"tools": tools}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error":   map[string]interface{}{"code": -32601, "message": "Method not found"},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  result,
	})
}