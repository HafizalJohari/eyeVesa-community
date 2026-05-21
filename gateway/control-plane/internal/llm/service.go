package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderLocal     Provider = "local"
)

type Config struct {
	Provider   Provider
	Model      string
	APIKey     string
	APIBaseURL string
	MaxTokens  int
	Temperature float64
}

func DefaultConfig() *Config {
	return &Config{
		Provider:    Provider(OrDefault("EYEVESA_LLM_PROVIDER", string(ProviderOpenAI))),
		Model:       OrDefault("EYEVESA_LLM_MODEL", "gpt-4"),
		APIKey:      os.Getenv("EYEVESA_LLM_API_KEY"),
		APIBaseURL:  OrDefault("EYEVESA_LLM_BASE_URL", "https://api.openai.com/v1"),
		MaxTokens:   1000,
		Temperature: 0.7,
	}
}

func OrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type LLMService struct {
	config *Config
	client *http.Client
}

func NewLLMService(config *Config) *LLMService {
	if config == nil {
		config = DefaultConfig()
	}
	return &LLMService{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type HITLSummaryRequest struct {
	AgentID       string                 `json:"agent_id"`
	AgentName     string                 `json:"agent_name"`
	TrustScore    float64                `json:"trust_score"`
	Action        string                 `json:"action"`
	ResourceID   string                 `json:"resource_id"`
	ResourceName string                 `json:"resource_name,omitempty"`
	RiskLevel     string                 `json:"risk_level"`
	Params        map[string]interface{} `json:"params"`
	Reason        string                 `json:"reason"`
	History       []ActionHistory        `json:"history,omitempty"`
}

type ActionHistory struct {
	Action     string    `json:"action"`
	Status     string    `json:"status"`
	TrustDelta float64  `json:"trust_delta"`
	Timestamp  time.Time `json:"timestamp"`
}

type HITLSummaryResponse struct {
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
	ApprovalID     string `json:"approval_id,omitempty"`
	TokensUsed    int    `json:"tokens_used"`
	ModelUsed      string `json:"model_used"`
}

func (s *LLMService) GenerateHITLSummary(ctx context.Context, req HITLSummaryRequest) (*HITLSummaryResponse, error) {
	prompt := fmt.Sprintf(`You are an approval assistant for AI agent actions. Generate a clear, concise summary for a human approver.

Agent: %s (ID: %s, trust: %.2f)
Action: %s
Resource: %s
Risk Level: %s
Parameters: %v
Reason: %s
Recent Actions: %d

Provide:
1. A 2-3 sentence summary of what this agent wants to do
2. A recommendation (APPROVE or DENY) based on trust score, risk, and history

Format as JSON: {"summary": "...", "recommendation": "APPROVE" or "DENY"}`,
		req.AgentName, req.AgentID, req.TrustScore, req.Action,
		req.ResourceID, req.RiskLevel, req.Params, req.Reason, len(req.History))

	resp, tokens, err := s.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM HITL summary: %w", err)
	}

	var result struct {
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &result); err != nil {
		result.Summary = resp
		result.Recommendation = "REVIEW"
	}

	return &HITLSummaryResponse{
		Summary:        result.Summary,
		Recommendation: result.Recommendation,
		TokensUsed:    tokens,
		ModelUsed:     s.config.Model,
	}, nil
}

type AuditNarrativeRequest struct {
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Events     []AuditEvent `json:"events"`
}

type AuditEvent struct {
	Action    string  `json:"action"`
	Status    string  `json:"status"`
	TrustDelta float64 `json:"trust_delta"`
	Timestamp time.Time `json:"timestamp"`
}

type AuditNarrativeResponse struct {
	NarrativeText  string `json:"narrative_text"`
	KeyEvents     []string `json:"key_events"`
	AnomaliesDetected int `json:"anomalies_detected"`
	TrustTrend    string `json:"trust_trend"`
	TokensUsed    int    `json:"tokens_used"`
	ModelUsed      string `json:"model_used"`
}

func (s *LLMService) GenerateAuditNarrative(ctx context.Context, req AuditNarrativeRequest) (*AuditNarrativeResponse, error) {
	eventsJSON, _ := json.Marshal(req.Events)

	prompt := fmt.Sprintf(`You are an audit analyst. Generate a human-readable narrative summary of agent activity.

Agent: %s (ID: %s)
Period: %s to %s
Events: %s

Provide:
1. A narrative summary (3-5 sentences) of what happened
2. Key events list
3. Anomaly count
4. Trust trend direction (improving/stable/degrading)

Format as JSON: {"narrative": "...", "key_events": [...], "anomalies": N, "trust_trend": "improving|stable|degrading"}`,
		req.AgentName, req.AgentID,
		req.PeriodStart.Format(time.RFC3339), req.PeriodEnd.Format(time.RFC3339),
		string(eventsJSON))

	resp, tokens, err := s.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM audit narrative: %w", err)
	}

	var result struct {
		Narrative string   `json:"narrative"`
		KeyEvents []string `json:"key_events"`
		Anomalies int      `json:"anomalies"`
		TrustTrend string  `json:"trust_trend"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &result); err != nil {
		result.Narrative = resp
		result.TrustTrend = "unknown"
	}

	return &AuditNarrativeResponse{
		NarrativeText:     result.Narrative,
		KeyEvents:        result.KeyEvents,
		AnomaliesDetected: result.Anomalies,
		TrustTrend:       result.TrustTrend,
		TokensUsed:      tokens,
		ModelUsed:        s.config.Model,
	}, nil
}

type PolicyTranslationRequest struct {
	NaturalLanguage string `json:"natural_language"`
	TenantID       string `json:"tenant_id"`
	ExistingRego   string `json:"existing_rego,omitempty"`
}

type PolicyTranslationResponse struct {
	GeneratedRego    string `json:"generated_rego"`
	Status           string `json:"status"`
	Validated        bool   `json:"validated"`
	ValidationErrors string `json:"validation_errors,omitempty"`
	TokensUsed      int    `json:"tokens_used"`
	ModelUsed        string `json:"model_used"`
}

func (s *LLMService) TranslatePolicy(ctx context.Context, req PolicyTranslationRequest) (*PolicyTranslationResponse, error) {
	prompt := fmt.Sprintf(`You are a Rego policy expert. Convert the following natural language policy into valid Rego code for an OPA policy engine.

Natural language: "%s"

Existing Rego policy context:
%s

Requirements:
- Use package agentid.authz
- Use import rego.v1
- Rules must return boolean values
- Include clear comments
- Follow OPA best practices

Return ONLY the Rego code, no explanation.`,
		req.NaturalLanguage, fallback(req.ExistingRego, "// No existing policy context"))

	resp, tokens, err := s.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM policy translation: %w", err)
	}

	generatedRego := resp
	if idx := bytes.Index([]byte(generatedRego), []byte("package ")); idx > 0 {
		generatedRego = generatedRego[idx:]
	}

	return &PolicyTranslationResponse{
		GeneratedRego:    generatedRego,
		Status:          "draft",
		Validated:       false,
		ValidationErrors: "",
		TokensUsed:     tokens,
		ModelUsed:       s.config.Model,
	}, nil
}

func (s *LLMService) callLLM(ctx context.Context, prompt string) (string, int, error) {
	if s.config.APIKey == "" {
		return s.localResponse(prompt), 0, nil
	}

	switch s.config.Provider {
	case ProviderOpenAI:
		return s.callOpenAI(ctx, prompt)
	case ProviderAnthropic:
		return s.callAnthropic(ctx, prompt)
	default:
		return s.callOpenAI(ctx, prompt)
	}
}

func (s *LLMService) callOpenAI(ctx context.Context, prompt string) (string, int, error) {
	payload := map[string]interface{}{
		"model":       s.config.Model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens":  s.config.MaxTokens,
		"temperature": s.config.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.config.APIBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("OpenAI API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *LLMService) callAnthropic(ctx context.Context, prompt string) (string, int, error) {
	payload := map[string]interface{}{
		"model":      s.config.Model,
		"max_tokens":  s.config.MaxTokens,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("Anthropic API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, err
	}

	if len(result.Content) == 0 {
		return "", 0, fmt.Errorf("no response from Anthropic")
	}

	totalTokens := result.Usage.InputTokens + result.Usage.OutputTokens
	return result.Content[0].Text, totalTokens, nil
}

func (s *LLMService) localResponse(prompt string) string {
	if len(prompt) > 2000 {
		prompt = prompt[:2000]
	}
	return `{"summary": "Agent requests action approval (local fallback - configure LLM API key for real summaries)", "recommendation": "REVIEW"}`
}

func extractJSON(s string) string {
	start := bytes.Index([]byte(s), []byte("{"))
	end := bytes.LastIndex([]byte(s), []byte("}"))
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return "{}"
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}