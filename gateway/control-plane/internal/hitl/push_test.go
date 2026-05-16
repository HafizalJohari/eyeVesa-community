package hitl

import (
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