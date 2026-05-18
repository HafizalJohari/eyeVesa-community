package hitl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
		BotToken: "123456:ABC-DEF",
		ChatID:   "-1001234567",
		Client:   server.Client(),
	}

	notifier.BotToken = "test-token"
	oldURL := "https://api.telegram.org"
	_ = oldURL

	ctx := context.Background()

	origClient := notifier.Client
	notifier.Client = &http.Client{Transport: &telegramTransport{serverURL: server.URL, botToken: "test-token"}}
	defer func() { notifier.Client = origClient }()

	err := notifier.Send(ctx, "", "Test HITL alert")
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

type telegramTransport struct {
	serverURL string
	botToken  string
}

func (t *telegramTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	orig := req.URL.String()
	_ = orig
	newReq := req.Clone(req.Context())
	newReq.URL, _ = req.URL.Parse(t.serverURL + req.URL.Path)
	return http.DefaultTransport.RoundTrip(newReq)
}