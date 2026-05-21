package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/database"
)

func TestListA2AAgents(t *testing.T) {
	q := &mockQuerier{
		queryFn: func(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
			return &mockRows{results: [][]interface{}{{"a1", "Agent One", "team-a", 0.99, "online", []string{"search"}}}}, nil
		},
	}
	r := setupTestRouter(q)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/a2a/agents")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if out["count"].(float64) != 1 {
		t.Fatalf("expected count=1, got %v", out["count"])
	}
}

func TestCreateAndGetA2ATask(t *testing.T) {
	r := setupTestRouter(&mockQuerier{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"from_agent_id": "parent-1",
		"to_agent_id":   "child-1",
		"action":        "summarize",
		"scope":         []string{"read:docs"},
	})

	resp, err := http.Post(ts.URL+"/v1/a2a/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create failed: %v", err)
	}
	taskID := created["task_id"].(string)
	if taskID == "" {
		t.Fatal("task_id is empty")
	}

	getResp, err := http.Get(ts.URL + "/v1/a2a/tasks/" + taskID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}
}

func TestCreateA2ATaskValidation(t *testing.T) {
	r := setupTestRouter(&mockQuerier{})
	ts := httptest.NewServer(r)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{"action": "do"})
	resp, err := http.Post(ts.URL+"/v1/a2a/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
