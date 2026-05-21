package hitl

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type PushNotifier struct {
	apnsKeyID    string
	apnsTeamID   string
	apnsKey      *ecdsa.PrivateKey
	apnsBundleID string
	apnsEndpoint string

	fcmServerKey  string
	fcmProjectID  string
	fcmSAKeyPath  string
	fcmSAKey      *ecdsa.PrivateKey
	fcmSAEmail    string
	fcmToken      string
	fcmTokenExpiry time.Time
	fcmTokenMu    sync.Mutex

	client *http.Client
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

	if saKeyPath := os.Getenv("FCM_SA_KEY_PATH"); saKeyPath != "" {
		keyData, err := os.ReadFile(saKeyPath)
		if err == nil {
			saKey, email, err := parseServiceAccountKey(keyData)
			if err == nil {
				n.fcmSAKey = saKey
				n.fcmSAEmail = email
				n.fcmProjectID = os.Getenv("FCM_PROJECT_ID")
				slog.Info("FCM OAuth2 configured", "project_id", n.fcmProjectID, "email", email)
			} else {
				slog.Warn("failed to parse FCM service account key", "error", err)
			}
		}
	}

	if n.fcmSAKey == nil {
		if serverKey := os.Getenv("FCM_SERVER_KEY"); serverKey != "" {
			n.fcmServerKey = serverKey
			n.fcmProjectID = os.Getenv("FCM_PROJECT_ID")
			slog.Warn("FCM using deprecated server key auth, migrate to FCM_SA_KEY_PATH for OAuth2")
		}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
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
	if n.fcmSAKey != nil && n.fcmProjectID != "" {
		return n.sendFCMOAuth2(ctx, token, message)
	}

	if n.fcmServerKey != "" {
		return n.sendFCMLegacy(ctx, token, message)
	}

	return fmt.Errorf("FCM not configured: set FCM_SA_KEY_PATH+FCM_PROJECT_ID or FCM_SERVER_KEY")
}

func (n *PushNotifier) sendFCMOAuth2(ctx context.Context, token string, message string) error {
	if n.fcmSAKey == nil || n.fcmSAEmail == "" || n.fcmProjectID == "" {
		return fmt.Errorf("FCM OAuth2 not configured: set FCM_SA_KEY_PATH and FCM_PROJECT_ID")
	}

	accessToken, err := n.getFCMAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("fcm oauth2 token: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", n.fcmProjectID)

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

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("fcm oauth2 payload marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("fcm oauth2 request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm oauth2 send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm oauth2 returned status %d", resp.StatusCode)
	}

	return nil
}

func (n *PushNotifier) sendFCMLegacy(ctx context.Context, token string, message string) error {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
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

func (n *PushNotifier) getFCMAccessToken(ctx context.Context) (string, error) {
	n.fcmTokenMu.Lock()
	defer n.fcmTokenMu.Unlock()

	if n.fcmToken != "" && time.Now().Before(n.fcmTokenExpiry) {
		return n.fcmToken, nil
	}

	now := time.Now().Unix()
	expiry := now + 3600

	header := base64.RawURLEncoding.EncodeToString([]byte(
		`{"alg":"RS256","typ":"JWT"}`,
	))

	claimSet := fmt.Sprintf(
		`{"iss":"%s","scope":"https://www.googleapis.com/auth/firebase.messaging","aud":"https://oauth2.googleapis.com/token","iat":%d,"exp":%d}`,
		n.fcmSAEmail, now, expiry,
	)

	claims := base64.RawURLEncoding.EncodeToString([]byte(claimSet))
	signingInput := header + "." + claims

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, n.fcmSAKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("fcm jwt sign: %w", err)
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, 64)
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):], sBytes)

	signature := base64.RawURLEncoding.EncodeToString(sig)
	jwt := signingInput + "." + signature

	tokenReq := fmt.Sprintf(
		"grant_type=urn%%3Aietf%%3Aparams%%3Aoauth%%3Agrant-type%%3Ajwt-bearer&assertion=%s",
		jwt,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(tokenReq),
	)
	if err != nil {
		return "", fmt.Errorf("fcm token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm token fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fcm token returned status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("fcm token decode: %w", err)
	}

	n.fcmToken = tokenResp.AccessToken
	n.fcmTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	slog.Info("FCM OAuth2 access token refreshed", "expires_in", tokenResp.ExpiresIn)

	return n.fcmToken, nil
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

func parseServiceAccountKey(data []byte) (*ecdsa.PrivateKey, string, error) {
	var sa struct {
		PrivateKey  string `json:"private_key"`
		ClientEmail string `json:"client_email"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, "", fmt.Errorf("failed to parse service account JSON: %w", err)
	}

	if sa.Type != "service_account" {
		return nil, "", fmt.Errorf("not a service account key, type=%s", sa.Type)
	}

	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, "", fmt.Errorf("failed to decode PEM block from service account private_key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse PKCS8 key: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, "", fmt.Errorf("service account key is not ECDSA")
	}

	return ecKey, sa.ClientEmail, nil
}