package hitl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SlackNotifier struct {
	WebhookURL string
	Client     *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *SlackNotifier) Send(ctx context.Context, target string, message string) error {
	payload := map[string]interface{}{
		"text": message,
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": message,
				},
			},
			{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type":  "button",
						"text":  map[string]string{"type": "plain_text", "text": "Approve"},
						"style": "primary",
						"value": "approve",
					},
					{
						"type":  "button",
						"text":  map[string]string{"type": "plain_text", "text": "Deny"},
						"style": "danger",
						"value": "deny",
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("slack send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

type WebhookNotifier struct {
	Client *http.Client
}

func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type WebhookPayload struct {
	Event      string                 `json:"event"`
	ApprovalID string                `json:"approval_id"`
	AgentID    string                 `json:"agent_id"`
	Action     string                 `json:"action"`
	RiskLevel  string                 `json:"risk_level"`
	Message    string                 `json:"message"`
	Timestamp  string                 `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func (n *WebhookNotifier) Send(ctx context.Context, target string, message string) error {
	payload := WebhookPayload{
		Event:     "hitl_approval_required",
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-EyeVesa-Event", "hitl_approval")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

type PagerDutyNotifier struct {
	IntegrationKey string
	Client         *http.Client
}

func NewPagerDutyNotifier(integrationKey string) *PagerDutyNotifier {
	return &PagerDutyNotifier{
		IntegrationKey: integrationKey,
		Client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *PagerDutyNotifier) Send(ctx context.Context, target string, message string) error {
	payload := map[string]interface{}{
		"routing_key":  n.IntegrationKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":  message,
			"severity": "warning",
			"source":   "eyeVesa",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pagerduty marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://events.pagerduty.com/v2/enqueue", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pagerduty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("pagerduty send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pagerduty returned status %d", resp.StatusCode)
	}

	return nil
}

type TelegramNotifier struct {
	BotToken string
	ChatID   string
	Client   *http.Client
}

func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *TelegramNotifier) Send(ctx context.Context, target string, message string) error {
	chatID := target
	if chatID == "" {
		chatID = n.ChatID
	}
	if chatID == "" {
		return fmt.Errorf("telegram: no chat_id provided")
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram marshal: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}

	return nil
}

type DiscordNotifier struct {
	WebhookURL string
	Client     *http.Client
}

func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL: webhookURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *DiscordNotifier) Send(ctx context.Context, target string, message string) error {
	webhookURL := target
	if webhookURL == "" {
		webhookURL = n.WebhookURL
	}
	if webhookURL == "" {
		return fmt.Errorf("discord: no webhook_url provided")
	}

	payload := map[string]interface{}{
		"content": message,
		"embeds": []map[string]interface{}{
			{
				"title":       "eyeVesa HITL Approval Required",
				"description": message,
				"color":       16761035,
				"fields": []map[string]interface{}{
					{
						"name":   "Action Required",
						"value":  "Review and approve or deny this request",
						"inline": false,
					},
				},
				"footer": map[string]string{
					"text": "eyeVesa AgentID Gateway",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("discord send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	return nil
}