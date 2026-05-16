package hitl

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type PushNotifier struct {
	apnsKeyID    string
	apnsTeamID   string
	apnsKey      *ecdsa.PrivateKey
	apnsBundleID string
	apnsEndpoint string
	fcmServerKey string
	fcmProjectID string
	client       *http.Client
}

func NewPushNotifier() *PushNotifier {
	n := &PushNotifier{
		client: &http.Client{Timeout: 10 * time.Second},
	}

	if keyPath := os.Getenv("APNS_KEY_PATH"); keyPath != "" {
		keyData, err := os.ReadFile(keyPath)
		if err == nil {
			key, err := parseECPrivateKey(keyData)
			if err == nil {
				n.apnsKey = key
				n.apnsKeyID = os.Getenv("APNS_KEY_ID")
				n.apnsTeamID = os.Getenv("APNS_TEAM_ID")
				n.apnsBundleID = os.Getenv("APNS_BUNDLE_ID")
				if os.Getenv("APNS_PRODUCTION") == "true" {
					n.apnsEndpoint = "https://api.push.apple.com"
				} else {
					n.apnsEndpoint = "https://api.sandbox.push.apple.com"
				}
			}
		}
	}

	if serverKey := os.Getenv("FCM_SERVER_KEY"); serverKey != "" {
		n.fcmServerKey = serverKey
		n.fcmProjectID = os.Getenv("FCM_PROJECT_ID")
	}

	return n
}

func (n *PushNotifier) Send(ctx context.Context, target string, message string) error {
	if target == "" {
		return fmt.Errorf("push notification requires a device token or FCM topic")
	}

	if strings.HasPrefix(target, "apns:") {
		deviceToken := strings.TrimPrefix(target, "apns:")
		return n.sendAPNs(ctx, deviceToken, message)
	}

	if strings.HasPrefix(target, "fcm:") {
		token := strings.TrimPrefix(target, "fcm:")
		return n.sendFCM(ctx, token, message)
	}

	if n.apnsKey != nil {
		return n.sendAPNs(ctx, target, message)
	}

	return n.sendFCM(ctx, target, message)
}

func (n *PushNotifier) sendAPNs(ctx context.Context, deviceToken string, message string) error {
	if n.apnsKey == nil {
		return fmt.Errorf("APNs not configured: set APNS_KEY_PATH, APNS_KEY_ID, APNS_TEAM_ID, APNS_BUNDLE_ID")
	}

	jwt, err := n.buildAPNSJWT()
	if err != nil {
		return fmt.Errorf("apns jwt build: %w", err)
	}

	payload := map[string]interface{}{
		"aps": map[string]interface{}{
			"alert": map[string]string{
				"title": "AgentID Approval Required",
				"body":  message,
			},
			"sound":    "default",
			"category": "HITL_APPROVAL",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("apns payload marshal: %w", err)
	}

	url := fmt.Sprintf("%s/3/device/%s", n.apnsEndpoint, deviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+jwt)
	req.Header.Set("apns-topic", n.apnsBundleID)
	req.Header.Set("apns-push-type", "alert")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("apns send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apns returned status %d", resp.StatusCode)
	}

	return nil
}

func (n *PushNotifier) sendFCM(ctx context.Context, token string, message string) error {
	if n.fcmServerKey == "" {
		return fmt.Errorf("FCM not configured: set FCM_SERVER_KEY")
	}

	var body []byte
	var url string

	if n.fcmProjectID != "" {
		url = fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", n.fcmProjectID)
		payload := map[string]interface{}{
			"message": map[string]interface{}{
				"token": token,
				"notification": map[string]string{
					"title": "AgentID Approval Required",
					"body":  message,
				},
				"android": map[string]interface{}{
					"priority": "high",
				},
				"data": map[string]string{
					"type": "hitl_approval",
				},
			},
		}
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("fcm payload marshal: %w", err)
		}
	} else {
		url = "https://fcm.googleapis.com/fcm/send"
		payload := map[string]interface{}{
			"to":          token,
			"notification": map[string]string{"title": "AgentID Approval Required", "body": message},
			"data":        map[string]string{"type": "hitl_approval"},
			"priority":     "high",
		}
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("fcm payload marshal: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+n.fcmServerKey)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm returned status %d", resp.StatusCode)
	}

	return nil
}

func (n *PushNotifier) buildAPNSJWT() (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"alg":"ES256","kid":"%s"}`, n.apnsKeyID),
	))

	now := time.Now().Unix()
	claims := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf(`{"iss":"%s","iat":%d}`, n.apnsTeamID, now),
	))

	signingInput := header + "." + claims
	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, n.apnsKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("apns jwt sign: %w", err)
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, 64)
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):], sBytes)

	signature := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + signature, nil
}

func parseECPrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, rest := pem.Decode(data)
	for block != nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			if ecKey, ok := key.(*ecdsa.PrivateKey); ok {
				return ecKey, nil
			}
		}
		block, rest = pem.Decode(rest)
	}

	key, err := x509.ParseECPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}
	return key, nil
}