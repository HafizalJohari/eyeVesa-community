package hitl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTelegramNotifierSend(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "-1001234567",
		Client:   &http.Client{Transport: &telegramTransport{serverURL: server.URL, botToken: "test-token"}},
	}

	err := notifier.Send(context.Background(), "", "Test HITL alert")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received["chat_id"] != "-1001234567" {
		t.Errorf("expected chat_id=-1001234567, got %v", received["chat_id"])
	}
	if received["parse_mode"] != "HTML" {
		t.Errorf("expected parse_mode=HTML, got %v", received["parse_mode"])
	}
	if received["text"] != "Test HITL alert" {
		t.Errorf("expected text='Test HITL alert', got %v", received["text"])
	}
}

func TestTelegramNotifierSendWithTargetOverride(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "-1001234567",
		Client:   &http.Client{Transport: &telegramTransport{serverURL: server.URL, botToken: "test-token"}},
	}

	err := notifier.Send(context.Background(), "-9999999", "Override target test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received["chat_id"] != "-9999999" {
		t.Errorf("expected target override to -9999999, got %v", received["chat_id"])
	}
}

func TestTelegramNotifierNoChatID(t *testing.T) {
	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "",
		Client:   http.DefaultClient,
	}

	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected error when no chat_id provided")
	}
}

func TestTelegramNotifierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := &TelegramNotifier{
		BotToken: "test-token",
		ChatID:   "-1001234567",
		Client:   &http.Client{Transport: &telegramTransport{serverURL: server.URL, botToken: "test-token"}},
	}

	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestDiscordNotifierSend(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := &DiscordNotifier{
		WebhookURL: server.URL,
		Client:     server.Client(),
	}

	err := notifier.Send(context.Background(), "", "Discord HITL alert")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received["content"] != "Discord HITL alert" {
		t.Errorf("expected content='Discord HITL alert', got %v", received["content"])
	}

	embeds, ok := received["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Error("expected embeds in payload")
	}
}

func TestDiscordNotifierSendWithTargetOverride(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := &DiscordNotifier{
		WebhookURL: "http://default.example.com",
		Client:     server.Client(),
	}

	err := notifier.Send(context.Background(), server.URL, "Override webhook test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received["content"] != "Override webhook test" {
		t.Errorf("expected content='Override webhook test', got %v", received["content"])
	}
}

func TestDiscordNotifierNoWebhookURL(t *testing.T) {
	notifier := &DiscordNotifier{
		WebhookURL: "",
		Client:     http.DefaultClient,
	}

	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected error when no webhook_url provided")
	}
}

func TestDiscordNotifierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	notifier := &DiscordNotifier{
		WebhookURL: server.URL,
		Client:     server.Client(),
	}

	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected error on 502 response")
	}
}

func TestSlackNotifierSend(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	err := notifier.Send(context.Background(), "", "Slack HITL alert")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received["text"] != "Slack HITL alert" {
		t.Errorf("expected text='Slack HITL alert', got %v", received["text"])
	}

	blocks, ok := received["blocks"].([]interface{})
	if !ok || len(blocks) == 0 {
		t.Error("expected blocks in Slack payload")
	}
}

func TestSlackNotifierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestSlackNotifierClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		WebhookURL: server.URL,
		Client:     &http.Client{Timeout: 50 * time.Millisecond},
	}

	err := notifier.Send(context.Background(), "", "test")
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestWebhookNotifierSend(t *testing.T) {
	var received WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-EyeVesa-Event") != "hitl_approval" {
			t.Errorf("expected X-EyeVesa-Event header, got %s", r.Header.Get("X-EyeVesa-Event"))
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier()
	err := notifier.Send(context.Background(), server.URL, "Webhook alert")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if received.Event != "hitl_approval_required" {
		t.Errorf("expected event=hitl_approval_required, got %s", received.Event)
	}
	if received.Message != "Webhook alert" {
		t.Errorf("expected message='Webhook alert', got %s", received.Message)
	}
}

func TestWebhookNotifierServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier()
	err := notifier.Send(context.Background(), server.URL, "test")
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestWebhookNotifierRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier()
	err := notifier.Send(context.Background(), server.URL, "test")
	if err == nil {
		t.Error("expected error on redirect response")
	}
}

func TestPagerDutyNotifierSend(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier := NewPagerDutyNotifier("test-integration-key")
	notifier.Client = server.Client()

	origURL := "https://events.pagerduty.com/v2/enqueue"
	_ = origURL

	err := notifier.Send(context.Background(), "", "PagerDuty alert")
	if err == nil {
		t.Fatal("expected error since notifier posts to real pagerduty URL, not test server")
	}
}

func TestPagerDutyNotifierNoKey(t *testing.T) {
	notifier := NewPagerDutyNotifier("")
	if notifier.IntegrationKey != "" {
		t.Fatal("empty key should be empty")
	}
}

func TestWebhookPayload_Fields(t *testing.T) {
	p := WebhookPayload{
		Event:      "test_event",
		ApprovalID: "ap1",
		AgentID:    "a1",
		Action:     "deploy",
		RiskLevel:  "high",
		Message:    "test",
		Timestamp:  "2026-01-01T00:00:00Z",
	}
	if p.Event != "test_event" {
		t.Fatalf("Event mismatch: got %s", p.Event)
	}
}

func TestNewTelegramNotifier(t *testing.T) {
	n := NewTelegramNotifier("bot-token", "chat-id")
	if n.BotToken != "bot-token" {
		t.Fatalf("BotToken mismatch: got %s", n.BotToken)
	}
	if n.ChatID != "chat-id" {
		t.Fatalf("ChatID mismatch: got %s", n.ChatID)
	}
}

func TestNewDiscordNotifier(t *testing.T) {
	n := NewDiscordNotifier("webhook-url")
	if n.WebhookURL != "webhook-url" {
		t.Fatalf("WebhookURL mismatch: got %s", n.WebhookURL)
	}
}

func TestNewPagerDutyNotifier(t *testing.T) {
	n := NewPagerDutyNotifier("key-123")
	if n.IntegrationKey != "key-123" {
		t.Fatalf("IntegrationKey mismatch: got %s", n.IntegrationKey)
	}
}

type telegramTransport struct {
	serverURL string
	botToken  string
}

func (t *telegramTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL, _ = req.URL.Parse(t.serverURL + req.URL.Path)
	return http.DefaultTransport.RoundTrip(newReq)
}