package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:9090")
	if c.BaseURL != "http://localhost:9090" {
		t.Errorf("BaseURL = %q, want http://localhost:9090", c.BaseURL)
	}
}

func TestNewClientEmpty(t *testing.T) {
	c := NewClient("")
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want http://localhost:8080", c.BaseURL)
	}
}

func TestClientHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Health()
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if result != "ok" {
		t.Errorf("Health() = %q, want ok", result)
	}
}

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path == "/v1/agents" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"agents": []interface{}{},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Get("/v1/agents")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if _, ok := result["agents"]; !ok {
		t.Error("Get() result missing 'agents' key")
	}
}

func TestClientPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path == "/v1/authorize" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowed":       true,
				"requires_hitl":  false,
				"reason":        "auto-approved",
				"trust_delta":    0.1,
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Post("/v1/authorize", map[string]interface{}{
		"agent_id": "test",
		"action":   "read",
	})
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	if allowed, _ := result["allowed"].(bool); !allowed {
		t.Error("Post() result allowed = false, want true")
	}
}

func TestClientAPIKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key-123" {
			t.Errorf("X-API-Key = %q, want test-key-123", r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.APIKey = "test-key-123"
	_, err := c.Get("/v1/agents")
	if err != nil {
		t.Fatalf("Get() with API key error: %v", err)
	}
}

func TestClientJWTHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-jwt-token" {
			t.Errorf("Authorization = %q, want Bearer test-jwt-token", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.JWTToken = "test-jwt-token"
	_, err := c.Get("/v1/agents")
	if err != nil {
		t.Fatalf("Get() with JWT error: %v", err)
	}
}

func TestClientErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "forbidden",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Get("/v1/agents")
	if err == nil {
		t.Fatal("Get() should return error for 403")
	}
}

func TestClientReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(200)
			w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() error: %v", err)
	}
	if status := result["status"].(int); status != 200 {
		t.Errorf("Ready() status = %v, want 200", status)
	}
}

func TestClientAuthorize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowed":       true,
			"requires_hitl":  false,
			"reason":        "allowed by policy",
			"trust_delta":    0.05,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Authorize("agent-1", "database_query", "db-1", nil)
	if err != nil {
		t.Fatalf("Authorize() error: %v", err)
	}
	if allowed, _ := result["allowed"].(bool); !allowed {
		t.Error("Authorize() result allowed = false, want true")
	}
}

func TestClientRegisterAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_id":    "agent-abc123",
			"public_key":  "dGVzdA==",
			"status":      "active",
			"trust_score": 0.5,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.RegisterAgent("test-agent", "org:eng", []string{"read"}, []string{"db_query"}, 100, "no_chain", []string{})
	if err != nil {
		t.Fatalf("RegisterAgent() error: %v", err)
	}
	if id, _ := result["agent_id"].(string); id != "agent-abc123" {
		t.Errorf("RegisterAgent() agent_id = %q, want agent-abc123", id)
	}
}