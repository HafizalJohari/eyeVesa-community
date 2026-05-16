package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		if msg, ok := result["error"].(string); ok {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return result, nil
}

func (c *Client) Get(path string) (map[string]interface{}, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

func (c *Client) Post(path string, body interface{}) (map[string]interface{}, error) {
	return c.doRequest(http.MethodPost, path, body)
}

func (c *Client) Delete(path string) (map[string]interface{}, error) {
	return c.doRequest(http.MethodDelete, path, nil)
}

func (c *Client) RegisterAgent(name, owner string, capabilities, allowedTools []string, maxBudgetUSD float64, delegationPolicy string, behavioralTags []string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":              name,
		"owner":             owner,
		"capabilities":      capabilities,
		"allowed_tools":     allowedTools,
		"max_budget_usd":    maxBudgetUSD,
		"delegation_policy": delegationPolicy,
		"behavioral_tags":   behavioralTags,
	}
	return c.Post("/v1/agents/register", body)
}

func (c *Client) GetAgent(agentID string) (map[string]interface{}, error) {
	return c.Get("/v1/agents/" + agentID)
}

func (c *Client) ListAgents() (map[string]interface{}, error) {
	return c.Get("/v1/agents")
}

func (c *Client) RegisterResource(name, resourceType, endpoint, authMethod, riskLevel, dataSensitivity string, rateLimit int, capabilities interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":                name,
		"type":               resourceType,
		"endpoint":           endpoint,
		"auth_method":        authMethod,
		"risk_level":         riskLevel,
		"data_sensitivity":   dataSensitivity,
		"rate_limit_per_agent": rateLimit,
		"capabilities":       capabilities,
	}
	return c.Post("/v1/resources/register", body)
}

func (c *Client) GetResource(resourceID string) (map[string]interface{}, error) {
	return c.Get("/v1/resources/" + resourceID)
}

func (c *Client) ListResources() (map[string]interface{}, error) {
	return c.Get("/v1/resources")
}

func (c *Client) Authorize(agentID, action, resourceID string, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	body := map[string]interface{}{
		"agent_id":    agentID,
		"action":      action,
		"resource_id": resourceID,
		"params":      params,
	}
	return c.Post("/v1/authorize", body)
}

func (c *Client) VerifySignature(agentID string, message, signature []byte) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id":  agentID,
		"message":   message,
		"signature": signature,
	}
	return c.Post("/v1/verify-signature", body)
}

func (c *Client) Health() (string, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/health")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) Delegate(parentAgentID, childAgentID string, scope []string, maxDepth int, duration string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"parent_agent_id": parentAgentID,
		"child_agent_id":  childAgentID,
		"scope":           scope,
		"max_depth":       maxDepth,
		"duration":        duration,
	}
	return c.Post("/v1/delegate", body)
}

func (c *Client) ListDelegations(agentID string) (map[string]interface{}, error) {
	return c.Get("/v1/delegations/" + agentID)
}

func (c *Client) ValidateDelegation(parentID, childID string) (map[string]interface{}, error) {
	return c.Get(fmt.Sprintf("/v1/delegations/validate?parent=%s&child=%s", parentID, childID))
}

func (c *Client) RevokeDelegation(delegationID string) (map[string]interface{}, error) {
	return c.Delete("/v1/delegations/" + delegationID)
}

func (c *Client) ListHILTPending() (map[string]interface{}, error) {
	return c.Get("/v1/hitl/pending")
}

func (c *Client) ApproveHILT(approvalID, approverMethod string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"approval_id":     approvalID,
		"approved":        true,
		"approver_method": approverMethod,
	}
	return c.Post("/v1/hitl/"+approvalID+"/decide", body)
}

func (c *Client) DenyHILT(approvalID, approverMethod string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"approval_id":     approvalID,
		"approved":        false,
		"approver_method": approverMethod,
	}
	return c.Post("/v1/hitl/"+approvalID+"/decide", body)
}

func (c *Client) Audit(agentID string, limit, offset int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/audit?agent_id=%s&limit=%d&offset=%d", agentID, limit, offset)
	return c.Get(path)
}

func (c *Client) MCPInitialize() (map[string]interface{}, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	}
	return c.Post("/v1/mcp", body)
}

func (c *Client) MCPToolsList() (map[string]interface{}, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	return c.Post("/v1/mcp", body)
}

func (c *Client) Identity() (map[string]interface{}, error) {
	return c.Get("/identity")
}