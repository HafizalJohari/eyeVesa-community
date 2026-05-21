package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

type gatewayClient struct {
	baseURL    string
	httpClient *http.Client
}

func newGatewayClient(endpoint string) *gatewayClient {
	base := endpoint
	if !strings.HasPrefix(base, "http") {
		base = "http://" + base
	}
	return &gatewayClient{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *gatewayClient) get(path string) (map[string]interface{}, error) {
	resp, err := g.httpClient.Get(g.baseURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func main() {
	resourceName := os.Getenv("RESOURCE_NAME")
	if resourceName == "" {
		resourceName = "demo-resource"
	}

	endpoint := os.Getenv("GATEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9443"
	}

	gw := newGatewayClient(endpoint)
	srv := server.New(resourceName, endpoint)

	srv.RegisterTool("airport_search", "Discover agents at the Airport by capability, trust score, and status. Returns KYA-verified agent profiles.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"capability": map[string]interface{}{
				"type":        "string",
				"description": "Filter by capability (e.g. research, trading, ops)",
			},
			"min_trust": map[string]interface{}{
				"type":        "number",
				"description": "Minimum trust score filter (0.0 - 1.0)",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"online", "offline", "busy", "idle"},
				"description": "Agent heartbeat status",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max results",
			},
		},
	}, func(params json.RawMessage) (interface{}, error) {
		var args struct {
			Capability string  `json:"capability"`
			MinTrust   float64 `json:"min_trust"`
			Status     string  `json:"status"`
			Limit      int     `json:"limit"`
		}
		json.Unmarshal(params, &args)
		q := ""
		if args.Capability != "" { q += "&capability=" + args.Capability }
		if args.MinTrust > 0 { q += fmt.Sprintf("&min_trust=%.2f", args.MinTrust) }
		if args.Status != "" { q += "&status=" + args.Status }
		if args.Limit > 0 { q += fmt.Sprintf("&limit=%d", args.Limit) }
		result, err := gw.get("/v1/airport/agents?" + strings.TrimPrefix(q, "&"))
		if err != nil {
			return nil, fmt.Errorf("airport unreachable: %w", err)
		}
		return result, nil
	})

	srv.RegisterTool("agent_trust", "Get KYA trust information for a specific agent. Returns trust score, approval rate, total actions, and current status.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "UUID of the agent to look up",
			},
		},
		"required": []string{"agent_id"},
	}, func(params json.RawMessage) (interface{}, error) {
		var args struct {
			AgentID string `json:"agent_id"`
		}
		json.Unmarshal(params, &args)
		if args.AgentID == "" {
			return nil, fmt.Errorf("agent_id is required")
		}
		result, err := gw.get("/v1/airport/agents/" + args.AgentID)
		if err != nil {
			return nil, fmt.Errorf("agent not found: %w", err)
		}
		return result, nil
	})

	srv.RegisterTool("audit_query", "Query the non-repudiable audit trail for an agent. Returns cryptographically signed action history with trust score changes.", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "Agent UUID to query audit for",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Number of log entries (default 20)",
			},
		},
		"required": []string{"agent_id"},
	}, func(params json.RawMessage) (interface{}, error) {
		var args struct {
			AgentID string `json:"agent_id"`
			Limit   int    `json:"limit"`
		}
		json.Unmarshal(params, &args)
		if args.Limit <= 0 { args.Limit = 20 }
		result, err := gw.get(fmt.Sprintf("/v1/audit/%s?limit=%d", args.AgentID, args.Limit))
		if err != nil {
			return nil, fmt.Errorf("audit query failed: %w", err)
		}
		return result, nil
	})

	srv.RegisterTool("airport_online", "List all agents currently online at the Airport with trust scores and status.", nil,
		func(params json.RawMessage) (interface{}, error) {
			result, err := gw.get("/v1/airport/online")
			if err != nil {
				return nil, fmt.Errorf("airport unreachable: %w", err)
			}
			return result, nil
		})

	srv.RegisterPrompt("delegate-task", "Generate a task delegation prompt for another agent at the Airport",
		func(args map[string]string) (string, error) {
			task := args["task"]
			targetAgent := args["target_agent"]
			if task == "" { task = "unknown task" }
			if targetAgent == "" { targetAgent = "unknown agent" }
			return fmt.Sprintf("I am delegating the following task to agent %s:\n\n%s\n\nPlease complete this task and report back with results, trust verification, and any issues encountered.", targetAgent, task), nil
		})

	srv.RegisterPrompt("compliance-report", "Generate a compliance report for an agent's actions over a period",
		func(args map[string]string) (string, error) {
			agentID := args["agent_id"]
			period := args["period"]
			if agentID == "" { agentID = "unknown agent" }
			if period == "" { period = "last 24 hours" }
			return fmt.Sprintf("Generate a compliance report for agent %s over %s.\n\nInclude:\n- Total actions performed\n- Approval rate\n- Trust score trend\n- Any policy violations\n- HITL interventions\n- Budget usage", agentID, period), nil
		})

	requiredSkills := os.Getenv("REQUIRED_SKILLS")
	if requiredSkills != "" {
		skills := strings.Split(requiredSkills, ",")
		for i, s := range skills {
			skills[i] = strings.TrimSpace(s)
		}
		srv.SetRequiredSkills(skills)
		log.Printf("Required skills set: %v", skills)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Resource Adapter '%s' starting, connecting to gateway at %s", resourceName, endpoint)

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}