package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
	JWTToken   string
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

	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	} else if c.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.JWTToken)
	}

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

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Support both old plain text "ok" and new JSON format
	if strings.TrimSpace(string(body)) == "ok" {
		return "ok", nil
	}

	var report map[string]interface{}
	if err := json.Unmarshal(body, &report); err == nil {
		if status, ok := report["status"].(string); ok && status == "healthy" {
			return "healthy", nil
		}
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

func (c *Client) Ready() (map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/ready")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return map[string]interface{}{
		"status": resp.StatusCode,
	}, nil
}

func (c *Client) RequestHITL(agentID, action, resourceID string, params map[string]interface{}, riskLevel string) (map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	body := map[string]interface{}{
		"agent_id":    agentID,
		"action":      action,
		"resource_id": resourceID,
		"params":      params,
		"risk_level":  riskLevel,
	}
	return c.Post("/v1/hitl/request", body)
}

func (c *Client) GetHITLStatus(approvalID string) (map[string]interface{}, error) {
	return c.Get("/v1/hitl/" + approvalID)
}

func (c *Client) EscalateHITL(agentID, action, resourceID, riskLevel, reason string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id":    agentID,
		"action":      action,
		"resource_id": resourceID,
		"risk_level":  riskLevel,
		"reason":      reason,
	}
	return c.Post("/v1/hitl/escalate", body)
}

func (c *Client) AttestPTV(agentID, platform, firmwareVersion string, tpmPublicKey []byte, runtimeHash []byte) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id":        agentID,
		"platform":        platform,
		"firmware_version": firmwareVersion,
		"tpm_public_key":  tpmPublicKey,
		"runtime_hash":    runtimeHash,
	}
	return c.Post("/v1/ptv/attest", body)
}

func (c *Client) BindPTV(attestationProof map[string]interface{}) (map[string]interface{}, error) {
	return c.Post("/v1/ptv/bind", attestationProof)
}

func (c *Client) VerifyPTV(bindingID string) (map[string]interface{}, error) {
	return c.Get("/v1/ptv/verify/" + bindingID)
}

func (c *Client) CheckBudget(agentID string) (map[string]interface{}, error) {
	return c.Get(fmt.Sprintf("/v1/budget/check?agent_id=%s", agentID))
}

func (c *Client) RecordSpend(agentID string, amount float64, currency, category string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id":  agentID,
		"amount":    amount,
		"currency":  currency,
		"category":  category,
	}
	return c.Post("/v1/budget/spend", body)
}

func (c *Client) CreateTenant(name, description string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
	}
	return c.Post("/v1/tenants", body)
}

func (c *Client) ListTenants() (map[string]interface{}, error) {
	return c.Get("/v1/tenants")
}

func (c *Client) GetTenant(tenantID string) (map[string]interface{}, error) {
	return c.Get("/v1/tenants/" + tenantID)
}

func (c *Client) RegisterPushToken(agentID, token, platform string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
		"token":    token,
		"platform": platform,
	}
	return c.Post("/v1/push/register", body)
}

func (c *Client) GetPushTokens() (map[string]interface{}, error) {
	return c.Get("/v1/push/tokens")
}

func (c *Client) DeactivatePushToken(tokenID string) (map[string]interface{}, error) {
	return c.Delete("/v1/push/tokens/" + tokenID)
}

func (c *Client) MCPCallTool(agentID, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	if arguments == nil {
		arguments = map[string]interface{}{}
	}
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	return c.Post("/v1/mcp", body)
}

func (c *Client) CreateTrustBundle(trustDomain, bundleData, bundleType, source, endpointURL string, isFederated bool) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"trust_domain":  trustDomain,
		"bundle_data":   bundleData,
		"bundle_type":   bundleType,
		"source":        source,
		"endpoint_url":  endpointURL,
		"is_federated":  isFederated,
	}
	return c.Post("/v1/spire/bundles", body)
}

func (c *Client) GetTrustBundle(trustDomain string) (map[string]interface{}, error) {
	return c.Get("/v1/spire/bundles/" + trustDomain)
}

func (c *Client) ListTrustBundles(federatedOnly bool) (map[string]interface{}, error) {
	path := "/v1/spire/bundles"
	if federatedOnly {
		path += "?federated=true"
	}
	return c.Get(path)
}

func (c *Client) UpdateTrustBundle(trustDomain, bundleData string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"bundle_data": bundleData,
	}
	return c.doRequest(http.MethodPut, "/v1/spire/bundles/"+trustDomain, body)
}

func (c *Client) VerifyTrustBundle(trustDomain string) (map[string]interface{}, error) {
	return c.Post("/v1/spire/bundles/"+trustDomain+"/verify", nil)
}

func (c *Client) DeleteTrustBundle(trustDomain string) (map[string]interface{}, error) {
	return c.Delete("/v1/spire/bundles/" + trustDomain)
}

func (c *Client) FetchBundleFromEndpoint(endpointURL, trustDomain string, save, isFederated bool) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"endpoint_url":  endpointURL,
		"trust_domain":  trustDomain,
		"save":          save,
		"is_federated":  isFederated,
	}
	return c.Post("/v1/spire/bundles/fetch", body)
}

func (c *Client) RegisterWorkload(spiffeID, agentID, trustDomain string, selectors []string, parentID string, autoRegister bool) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"spiffe_id":     spiffeID,
		"agent_id":      agentID,
		"trust_domain":  trustDomain,
		"selectors":     selectors,
		"parent_id":     parentID,
		"auto_register": autoRegister,
	}
	return c.Post("/v1/spire/workloads", body)
}

func (c *Client) GetWorkload(spiffeID string) (map[string]interface{}, error) {
	return c.Get("/v1/spire/workloads/" + spiffeID)
}

func (c *Client) ListWorkloads(agentID string) (map[string]interface{}, error) {
	path := "/v1/spire/workloads"
	if agentID != "" {
		path += "?agent_id=" + agentID
	}
	return c.Get(path)
}

func (c *Client) AttestWorkload(spiffeID string) (map[string]interface{}, error) {
	return c.Post("/v1/spire/workloads/"+spiffeID+"/attest", nil)
}

func (c *Client) DeleteWorkload(spiffeID string) (map[string]interface{}, error) {
	return c.Delete("/v1/spire/workloads/" + spiffeID)
}

func (c *Client) SpireStatus() (map[string]interface{}, error) {
	return c.Get("/v1/spire/status")
}

func (c *Client) Put(path string, body interface{}) (map[string]interface{}, error) {
	return c.doRequest(http.MethodPut, path, body)
}

func (c *Client) CreateSkill(name, description, category, riskLevel string, requiredTrustMin float64, requiredProficiency int) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":                  name,
		"description":          description,
		"category":             category,
		"risk_level":           riskLevel,
		"required_trust_min":   requiredTrustMin,
		"required_proficiency": requiredProficiency,
	}
	return c.Post("/v1/skills", body)
}

func (c *Client) GetSkill(skillID string) (map[string]interface{}, error) {
	return c.Get("/v1/skills/" + skillID)
}

func (c *Client) ListSkills(category string) (map[string]interface{}, error) {
	path := "/v1/skills"
	if category != "" {
		path += "?category=" + category
	}
	return c.Get(path)
}

func (c *Client) SearchSkills(query, category string) (map[string]interface{}, error) {
	path := "/v1/skills/search?q=" + query + "&category=" + category
	return c.Get(path)
}

func (c *Client) UpdateSkill(skillID, description, category, riskLevel string, requiredTrustMin float64, requiredProficiency int) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"description":          description,
		"category":             category,
		"risk_level":           riskLevel,
		"required_trust_min":   requiredTrustMin,
		"required_proficiency": requiredProficiency,
	}
	return c.Put("/v1/skills/"+skillID, body)
}

func (c *Client) DeleteSkill(skillID string) (map[string]interface{}, error) {
	return c.Delete("/v1/skills/" + skillID)
}

func (c *Client) AssignSkill(agentID, skillID string, proficiency int) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"skill_id":    skillID,
		"proficiency": proficiency,
	}
	return c.Post("/v1/agents/"+agentID+"/skills", body)
}

func (c *Client) RemoveSkill(agentID, skillID string) (map[string]interface{}, error) {
	return c.Delete("/v1/agents/" + agentID + "/skills/" + skillID)
}

func (c *Client) ListAgentSkills(agentID string) (map[string]interface{}, error) {
	return c.Get("/v1/agents/" + agentID + "/skills")
}

func (c *Client) VerifySkill(agentID, skillID, verifiedBy string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"verified_by": verifiedBy,
	}
	return c.Post("/v1/agents/"+agentID+"/skills/"+skillID+"/verify", body)
}

func (c *Client) EndorseSkill(agentID, skillID, endorserType, endorserID, comment string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"endorser_type": endorserType,
		"endorser_id":   endorserID,
		"comment":       comment,
	}
	return c.Post("/v1/agents/"+agentID+"/skills/"+skillID+"/endorse", body)
}

func (c *Client) ListEndorsements(agentID, skillID string) (map[string]interface{}, error) {
	path := "/v1/agents/" + agentID + "/skills/" + skillID + "/endorsements"
	return c.Get(path)
}

func (c *Client) GetSkillTrust(agentID, skillID string) (map[string]interface{}, error) {
	path := "/v1/agents/" + agentID + "/skill-trust"
	if skillID != "" {
		path += "?skill_id=" + skillID
	}
	return c.Get(path)
}

func (c *Client) AdjustSkillTrust(agentID, skillID string, delta float64, reason string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"delta":  delta,
		"reason": reason,
	}
	return c.Post("/v1/agents/"+agentID+"/skill-trust/"+skillID, body)
}

func (c *Client) CheckSkillAuthz(agentID, action string) (map[string]interface{}, error) {
	return c.Get("/v1/agents/" + agentID + "/skill-authz?action=" + action)
}

func (c *Client) FindMissingSkills(agentID string, requiredSkills []string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"required_skills": requiredSkills,
	}
	return c.Post("/v1/agents/"+agentID+"/missing-skills", body)
}

func (c *Client) IssueCapabilityToken(agentID, resourceID, action string, scopes []string, params map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id":    agentID,
		"resource_id": resourceID,
		"action":      action,
		"scopes":      scopes,
		"params":      params,
	}
	return c.Post("/v1/tx/issue", body)
}

func (c *Client) VerifyCapabilityToken(token string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"token": token,
	}
	return c.Post("/v1/tx/verify", body)
}

func (c *Client) RevokeCapabilityToken(tokenID, reason string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"reason": reason,
	}
	return c.Post("/v1/tx/revoke/"+tokenID, body)
}

func (c *Client) ListRevokedTokens() (map[string]interface{}, error) {
	return c.Get("/v1/tx/revoked")
}

func (c *Client) IssueTransactionReceipt(token string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"token": token,
	}
	return c.Post("/v1/tx/receipt", body)
}

func (c *Client) VerifyTransactionReceipt(receipt string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"receipt": receipt,
	}
	return c.Post("/v1/tx/receipt/verify", body)
}

func (c *Client) AirportHeartbeat(agentID, status string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"agent_id": agentID,
		"status":   status,
	}
	return c.Post("/v1/airport/heartbeat", body)
}

func (c *Client) AirportSearch(params map[string]interface{}) (map[string]interface{}, error) {
	query := "/v1/airport/agents?"
	parts := []string{}
	if v, ok := params["capability"].(string); ok && v != "" {
		parts = append(parts, "capability="+v)
	}
	if v, ok := params["skill"].(string); ok && v != "" {
		parts = append(parts, "skill="+v)
	}
	if v, ok := params["status"].(string); ok && v != "" {
		parts = append(parts, "status="+v)
	}
	if v, ok := params["tag"].(string); ok && v != "" {
		parts = append(parts, "tag="+v)
	}
	if v, ok := params["owner"].(string); ok && v != "" {
		parts = append(parts, "owner="+v)
	}
	if v, ok := params["min_trust"].(float64); ok {
		parts = append(parts, fmt.Sprintf("min_trust=%f", v))
	}
	limit := 50
	if v, ok := params["limit"].(int); ok && v > 0 {
		limit = v
	}
	parts = append(parts, fmt.Sprintf("limit=%d", limit))
	query += strings.Join(parts, "&")
	return c.Get(query)
}

func (c *Client) AirportGetProfile(agentID string) (map[string]interface{}, error) {
	return c.Get("/v1/airport/agents/" + agentID)
}

func (c *Client) AirportUpdateProfile(agentID string, update map[string]interface{}) (map[string]interface{}, error) {
	return c.Put("/v1/airport/agents/"+agentID, update)
}

func (c *Client) AirportListOnline() (map[string]interface{}, error) {
	return c.Get("/v1/airport/online")
}

func (c *Client) AirportConnections(agentID string, limit int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/airport/connections?agent_id=%s&limit=%d", agentID, limit)
	return c.Get(path)
}