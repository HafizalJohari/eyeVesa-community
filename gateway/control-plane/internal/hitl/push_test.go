package hitl

import (
	"encoding/pem"
	"net/http"
	"testing"
)

func TestPushNotifierAPNsConfig(t *testing.T) {
	notifier := NewPushNotifier()
	if notifier == nil {
		t.Fatal("NewPushNotifier returned nil")
	}
}

func TestPushNotifierSendEmptyTarget(t *testing.T) {
	notifier := NewPushNotifier()
	err := notifier.Send(nil, "", "test message")
	if err == nil {
		t.Fatal("Send with empty target should fail")
	}
}

func TestPushNotifierAPNsPrefix(t *testing.T) {
	notifier := NewPushNotifier()
	err := notifier.Send(nil, "apns:device123", "test")
	if err == nil {
		t.Fatal("Send with APNs prefix should fail when APNs not configured")
	}
}

func TestPushNotifierFCMPrefix(t *testing.T) {
	notifier := NewPushNotifier()
	err := notifier.Send(nil, "fcm:token123", "test")
	if err == nil {
		t.Fatal("Send with FCM prefix should fail when FCM not configured")
	}
}

func TestParseServiceAccountKeyInvalidJSON(t *testing.T) {
	_, _, err := parseServiceAccountKey([]byte("not json"))
	if err == nil {
		t.Fatal("should fail for invalid JSON")
	}
}

func TestParseServiceAccountKeyWrongType(t *testing.T) {
	saJSON := `{"type":"user","client_email":"test@test.iam.gserviceaccount.com","private_key":""}`
	_, _, err := parseServiceAccountKey([]byte(saJSON))
	if err == nil {
		t.Fatal("should fail for non-service-account type")
	}
}

func TestParseServiceAccountKeyNoPEM(t *testing.T) {
	saJSON := `{"type":"service_account","client_email":"test@test.iam.gserviceaccount.com","private_key":"not-a-pem"}`
	_, _, err := parseServiceAccountKey([]byte(saJSON))
	if err == nil {
		t.Fatal("should fail for non-PEM private key")
	}
}

func TestParseServiceAccountKeyValidPEMButNotECDSA(t *testing.T) {
	rsaPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: []byte("invalid-pkcs8-data"),
	}))
	saJSON := `{"type":"service_account","client_email":"test@test.iam.gserviceaccount.com","private_key":"` + rsaPEM + `"}`
	_, _, err := parseServiceAccountKey([]byte(saJSON))
	if err == nil {
		t.Fatal("should fail for non-ECDSA key data")
	}
}

func TestFCMOAuth2NotConfigured(t *testing.T) {
	notifier := &PushNotifier{client: newTestHTTPClient()}
	err := notifier.sendFCMOAuth2(nil, "token123", "test message")
	if err == nil {
		t.Fatal("should fail when OAuth2 not configured")
	}
}

func TestFCMLegacyNotConfigured(t *testing.T) {
	notifier := &PushNotifier{client: newTestHTTPClient()}
	err := notifier.sendFCMLegacy(nil, "token123", "test message")
	if err == nil {
		t.Fatal("should fail when legacy FCM not configured")
	}
}

func TestPushNotifierSendAPNsNotConfigured(t *testing.T) {
	notifier := NewPushNotifier()
	err := notifier.sendAPNs(nil, "device-token", "test")
	if err == nil {
		t.Fatal("should fail when APNs not configured")
	}
}

func TestPushNotifierSendFCMNotConfigured(t *testing.T) {
	notifier := NewPushNotifier()
	err := notifier.sendFCM(nil, "fcm-token", "test")
	if err == nil {
		t.Fatal("should fail when FCM not configured")
	}
}

func TestPushToken_Fields(t *testing.T) {
	pt := PushToken{
		TokenID:     "t1",
		ApproverID:  "a1",
		DeviceToken: "device-abc",
		Platform:    "ios",
		BundleID:    "com.test.app",
		IsActive:    true,
	}
	if pt.TokenID != "t1" {
		t.Fatalf("TokenID mismatch: got %s", pt.TokenID)
	}
	if !pt.IsActive {
		t.Fatal("IsActive should be true")
	}
}

func TestPushService_NewWithNilDB(t *testing.T) {
	svc := NewPushService(nil)
	if svc == nil {
		t.Fatal("NewPushService should return non-nil")
	}
}

func TestParseECPrivateKeyInvalid(t *testing.T) {
	_, err := parseECPrivateKey([]byte("not-a-key"))
	if err == nil {
		t.Fatal("should fail for invalid key data")
	}
}

func TestParseECPrivateKeyEmpty(t *testing.T) {
	_, err := parseECPrivateKey([]byte(""))
	if err == nil {
		t.Fatal("should fail for empty key data")
	}
}

func newTestHTTPClient() *http.Client {
	return &http.Client{Timeout: 1}
}