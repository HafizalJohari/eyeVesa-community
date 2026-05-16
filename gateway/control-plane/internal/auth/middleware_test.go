package auth

import (
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key1 := GenerateAPIKey()
	key2 := GenerateAPIKey()
	if key1 == key2 {
		t.Fatal("Two generated API keys should not be equal")
	}
	if len(key1) < 20 {
		t.Fatalf("API key too short: %s", key1)
	}
}

func TestGenerateJWTSecret(t *testing.T) {
	secret1 := GenerateJWTSecret()
	secret2 := GenerateJWTSecret()
	if string(secret1) == string(secret2) {
		t.Fatal("Two generated JWT secrets should not be equal")
	}
	if len(secret1) < 32 {
		t.Fatalf("JWT secret too short: got %d bytes", len(secret1))
	}
}

func TestParseJWTValid(t *testing.T) {
	secret := GenerateJWTSecret()
	token := buildJWTToken(&JWTClaims{
		TenantID:  "test-tenant",
		Email:    "test@example.com",
		Role:     "admin",
		ExpiresAt: 9999999999,
		IssuedAt:  1000000000,
	}, secret)

	claims, err := parseJWT(token, secret)
	if err != nil {
		t.Fatalf("parseJWT failed: %v", err)
	}
	if claims.TenantID != "test-tenant" {
		t.Fatalf("TenantID mismatch: got %s", claims.TenantID)
	}
	if claims.Email != "test@example.com" {
		t.Fatalf("Email mismatch: got %s", claims.Email)
	}
	if claims.Role != "admin" {
		t.Fatalf("Role mismatch: got %s", claims.Role)
	}
}

func TestParseJWTWrongSecret(t *testing.T) {
	secret1 := GenerateJWTSecret()
	secret2 := GenerateJWTSecret()
	token := buildJWTToken(&JWTClaims{
		TenantID:  "test",
		ExpiresAt: 9999999999,
		IssuedAt:  1000000000,
	}, secret1)

	_, err := parseJWT(token, secret2)
	if err == nil {
		t.Fatal("parseJWT should fail with wrong secret")
	}
}

func TestParseJWTExpired(t *testing.T) {
	secret := GenerateJWTSecret()
	token := buildJWTToken(&JWTClaims{
		TenantID:  "test",
		ExpiresAt: 1,
		IssuedAt:  1,
	}, secret)

	_, err := parseJWT(token, secret)
	if err == nil {
		t.Fatal("parseJWT should reject expired token")
	}
}

func TestParseJWTInvalidFormat(t *testing.T) {
	_, err := parseJWT("not-a-jwt", []byte("secret"))
	if err == nil {
		t.Fatal("parseJWT should fail for invalid format")
	}
}

func TestIsPublicPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/identity", true},
		{"/v1/agents/register", true},
		{"/v1/resources/register", true},
		{"/v1/mcp", true},
		{"/v1/authorize", false},
		{"/v1/hitl/request", false},
		{"/v1/delegate", false},
	}
	for _, tt := range tests {
		if got := isPublicPath(tt.path); got != tt.expected {
			t.Errorf("isPublicPath(%q) = %v, want %v", tt.path, got, tt.expected)
		}
	}
}