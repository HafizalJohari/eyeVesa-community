package tenant

import (
	"testing"
)

func TestNewTenantService(t *testing.T) {
	svc := NewTenantService(nil)
	if svc == nil {
		t.Fatal("NewTenantService returned nil")
	}
}