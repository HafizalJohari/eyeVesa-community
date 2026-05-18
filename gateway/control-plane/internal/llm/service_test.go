package llm

import (
	"os"
	"strings"
	"testing"
)

func TestOrDefault_EnvSet(t *testing.T) {
	os.Setenv("TEST_LLM_VAR", "custom-value")
	defer os.Unsetenv("TEST_LLM_VAR")

	if got := OrDefault("TEST_LLM_VAR", "default"); got != "custom-value" {
		t.Fatalf("expected custom-value, got %s", got)
	}
}

func TestOrDefault_EnvNotSet(t *testing.T) {
	if got := OrDefault("NONEXISTENT_VAR_12345", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.MaxTokens != 1000 {
		t.Fatalf("MaxTokens should be 1000, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Fatalf("Temperature should be 0.7, got %f", cfg.Temperature)
	}
}

func TestNewLLMService_DefaultConfig(t *testing.T) {
	svc := NewLLMService(nil)
	if svc == nil {
		t.Fatal("NewLLMService returned nil")
	}
	if svc.config == nil {
		t.Fatal("config should not be nil")
	}
	if svc.client == nil {
		t.Fatal("client should not be nil")
	}
}

func TestNewLLMService_CustomConfig(t *testing.T) {
	cfg := &Config{
		Provider:   ProviderOpenAI,
		Model:      "gpt-4-turbo",
		APIKey:     "test-key",
		MaxTokens:  500,
		Temperature: 0.5,
	}
	svc := NewLLMService(cfg)
	if svc.config.Model != "gpt-4-turbo" {
		t.Fatalf("Model mismatch: got %s", svc.config.Model)
	}
	if svc.config.MaxTokens != 500 {
		t.Fatalf("MaxTokens mismatch: got %d", svc.config.MaxTokens)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`here is some text {"key": "val"} more text`, `{"key": "val"}`},
		{`{"key": "val"}`, `{"key": "val"}`},
		{`no json here`, `{}`},
		{`prefix {"a":1} middle {"b":2} suffix`, `{"a":1} middle {"b":2}`},
		{``, `{}`},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.expected {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFallback(t *testing.T) {
	if got := fallback("", "default"); got != "default" {
		t.Fatalf("empty string should use default, got %s", got)
	}
	if got := fallback("value", "default"); got != "value" {
		t.Fatalf("non-empty should use value, got %s", got)
	}
}

func TestLocalResponse(t *testing.T) {
	svc := NewLLMService(&Config{APIKey: ""})
	resp := svc.localResponse("test prompt")
	if resp == "" {
		t.Fatal("localResponse should not be empty")
	}
	if !strings.Contains(resp, "local fallback") {
		t.Fatalf("localResponse should mention local fallback, got: %s", resp)
	}
}

func TestLocalResponse_LongPrompt(t *testing.T) {
	svc := NewLLMService(&Config{APIKey: ""})
	longPrompt := make([]byte, 5000)
	for i := range longPrompt {
		longPrompt[i] = 'a'
	}
	resp := svc.localResponse(string(longPrompt))
	if resp == "" {
		t.Fatal("localResponse should handle long prompts")
	}
}

func TestCallLLM_NoAPIKey(t *testing.T) {
	svc := NewLLMService(&Config{APIKey: ""})
	resp, tokens, err := svc.callLLM(nil, "test")
	if err != nil {
		t.Fatalf("callLLM with no key should use local: %v", err)
	}
	if tokens != 0 {
		t.Fatalf("local response tokens should be 0, got %d", tokens)
	}
	if resp == "" {
		t.Fatal("response should not be empty")
	}
}

func TestProviderConstants(t *testing.T) {
	if ProviderOpenAI != "openai" {
		t.Fatalf("ProviderOpenAI should be 'openai', got %s", ProviderOpenAI)
	}
	if ProviderAnthropic != "anthropic" {
		t.Fatalf("ProviderAnthropic should be 'anthropic', got %s", ProviderAnthropic)
	}
	if ProviderLocal != "local" {
		t.Fatalf("ProviderLocal should be 'local', got %s", ProviderLocal)
	}
}

func TestHITLSummaryRequest_Fields(t *testing.T) {
	req := HITLSummaryRequest{
		AgentID:    "a1",
		AgentName:  "Test Agent",
		TrustScore: 0.85,
		Action:     "deploy",
		RiskLevel:  "high",
	}
	if req.AgentID != "a1" {
		t.Fatalf("AgentID mismatch: got %s", req.AgentID)
	}
}

func TestAuditEvent_Fields(t *testing.T) {
	e := AuditEvent{
		Action:     "deploy",
		Status:     "approved",
		TrustDelta: 0.01,
	}
	if e.Action != "deploy" {
		t.Fatalf("Action mismatch: got %s", e.Action)
	}
}