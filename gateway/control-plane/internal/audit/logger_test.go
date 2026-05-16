package audit

import (
	"testing"
)

func TestNewAuditLogger(t *testing.T) {
	logger := NewAuditLogger(nil)
	if logger == nil {
		t.Fatal("NewAuditLogger returned nil")
	}
}