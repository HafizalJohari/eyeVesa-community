package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
	resourceName := os.Getenv("RESOURCE_NAME")
	if resourceName == "" {
		resourceName = "demo-resource"
	}

	endpoint := os.Getenv("GATEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9443"
	}

	srv := server.New(resourceName, endpoint)

	srv.RegisterTool("get_weather", "Get current weather for a location", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "City name or coordinates",
			},
		},
		"required": []string{"location"},
	}, func(params json.RawMessage) (interface{}, error) {
		var args struct {
			Location string `json:"location"`
		}
		json.Unmarshal(params, &args)
		return map[string]interface{}{
			"location": args.Location,
			"temp":     "22C",
			"condition": "sunny",
		}, nil
	})

	srv.RegisterTool("search_docs", "Search documentation for a query", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
		},
		"required": []string{"query"},
	}, func(params json.RawMessage) (interface{}, error) {
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(params, &args)
		return map[string]interface{}{
			"query": args.Query,
			"results": []string{
				fmt.Sprintf("Documentation result for: %s", args.Query),
			},
		}, nil
	})

	srv.RegisterResource("docs://api-reference", "API Reference", "Complete API documentation for the gateway", "application/json",
		func(uri string) (interface{}, error) {
			return map[string]interface{}{
				"endpoints": []string{"/v1/agents/register", "/v1/auth", "/v1/mcp"},
				"version":   "0.1.0",
			}, nil
		})

	srv.RegisterResource("docs://trust-model", "Trust Model", "How the AgentID trust scoring system works", "text/markdown",
		func(uri string) (interface{}, error) {
			return "# Trust Model\n\nTrust scores start at 1.0 and adjust based on actions:\n- Allowed: +0.01\n- Denied: -0.05\n- Cost over budget: -0.10", nil
		})

	srv.RegisterPrompt("summarize", "Summarize the given text concisely",
		func(args map[string]string) (string, error) {
			text := args["text"]
			if text == "" {
				text = "No text provided"
			}
			return fmt.Sprintf("Please summarize the following text in 2-3 sentences:\n\n%s", text), nil
		})

	srv.RegisterPrompt("analyze-risk", "Analyze the risk level of a proposed action",
		func(args map[string]string) (string, error) {
			action := args["action"]
			if action == "" {
				action = "unknown action"
			}
			return fmt.Sprintf("Analyze the risk level of this proposed action:\n%s\n\nConsider: data sensitivity, financial impact, reversibility, and regulatory compliance.", action), nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Resource Adapter '%s' starting, connecting to gateway at %s", resourceName, endpoint)

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}