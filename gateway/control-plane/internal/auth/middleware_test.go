package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	if !strings.HasPrefix(key1, "eyevesa_") {
		t.Fatalf("API key should have eyevesa_ prefix, got: %s", key1)
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
		Email:     "test@example.com",
		Role:      "admin",
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
		method   string
		path     string
		expected bool
	}{
		{"GET", "/health", true},
		{"GET", "/identity", true},
		{"GET", "/ready", true},
		{"GET", "/metrics", true},
		{"POST", "/v1/agents/register", false},
		{"POST", "/v1/resources/register", true},
		{"POST", "/v1/mcp", true},
		{"GET", "/v1/api-keys", false},
		{"POST", "/v1/auth/challenge", true},
		{"POST", "/v1/auth/login", true},
		{"POST", "/v1/authorize", false},
		{"POST", "/v1/hitl/request", false},
		{"POST", "/v1/delegate", false},
		{"GET", "/v1/airport/health", true},
		{"GET", "/v1/airport/online", true},
		{"GET", "/v1/airport/agents", true},
		{"GET", "/v1/airport/agents/uuid-123", true},
		{"POST", "/v1/airport/heartbeat", false},
		{"GET", "/v1/airport/stats", true},
		{"PUT", "/v1/airport/agents/uuid-123", false},
		{"GET", "/v1/airport/connections", false},
	}
	for _, tt := range tests {
		if got := isPublicPath(tt.method, tt.path); got != tt.expected {
			t.Errorf("isPublicPath(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.expected)
		}
	}
}

func TestMiddleware_PublicPath(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	publicPaths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodGet, "/identity"},
		{http.MethodPost, "/v1/auth/challenge"},
		{http.MethodPost, "/v1/auth/login"},
		{http.MethodPost, "/v1/resources/register"},
		{http.MethodPost, "/v1/mcp"},
		{http.MethodGet, "/v1/airport/stats"},
	}
	for _, tt := range publicPaths {
		called = false
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if !called {
			t.Errorf("public path %s %s should pass through", tt.method, tt.path)
		}
	}
}

func TestMiddleware_Unauthorized(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call next handler for unauthorized request")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_BearerToken(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	called := false
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	token := buildJWTToken(&JWTClaims{
		TenantID:  "t1",
		Email:     "u@test.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("bearer token should pass through")
	}
}

func TestMiddleware_BearerToken_InjectsContext(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	var gotTenant, gotRole, gotEmail string
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = GetTenantID(r.Context())
		gotRole = GetRole(r.Context())
		gotEmail = GetEmail(r.Context())
	}))

	token := buildJWTToken(&JWTClaims{
		TenantID:  "tenant-42",
		Email:     "admin@example.com",
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotTenant != "tenant-42" {
		t.Fatalf("expected tenant-42, got %s", gotTenant)
	}
	if gotRole != "admin" {
		t.Fatalf("expected admin, got %s", gotRole)
	}
	if gotEmail != "admin@example.com" {
		t.Fatalf("expected admin@example.com, got %s", gotEmail)
	}
}

func TestMiddleware_ProtectedPaths_RequireAuth(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("protected paths should require authentication")
	}))

	protectedPaths := []string{
		"/v1/authorize",
		"/v1/delegate",
		"/v1/agents",
		"/v1/hitl/request",
		"/v1/api-keys",
	}

	for _, path := range protectedPaths {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s, got %d", path, rec.Code)
		}
	}
}

func TestMiddleware_ExpiredBearerToken(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expired token should not pass")
	}))

	token := buildJWTToken(&JWTClaims{
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		IssuedAt:  time.Now().Add(-2 * time.Hour).Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidBearerToken(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("invalid token should not pass")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestMiddleware_SSO(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	var gotTenant string
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = GetTenantID(r.Context())
	}))

	token := buildJWTToken(&JWTClaims{
		TenantID:  "tenant-abc",
		Role:      "approver",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.AddCookie(&http.Cookie{Name: "eyevesa_sso", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotTenant != "tenant-abc" {
		t.Fatalf("expected tenant-abc, got %s", gotTenant)
	}
}

func TestMiddleware_ExpiredSSO(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expired SSO should not pass")
	}))

	token := buildJWTToken(&JWTClaims{
		TenantID:  "tenant-abc",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		IssuedAt:  time.Now().Add(-2 * time.Hour).Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.AddCookie(&http.Cookie{Name: "eyevesa_sso", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired SSO, got %d", rec.Code)
	}
}

func TestMiddleware_SSONoTenantID(t *testing.T) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("SSO without tenant_id should not pass")
	}))

	token := buildJWTToken(&JWTClaims{
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	req.AddCookie(&http.Cookie{Name: "eyevesa_sso", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_AdminSucceeds(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	called := false
	handler := auth.RequireRole("operator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
	ctx := context.WithValue(req.Context(), roleCtxKey{}, "admin")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("admin should pass operator role check")
	}
}

func TestRequireRole_ViewerFails(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	handler := auth.RequireRole("operator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("viewer should not pass operator role check")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
	ctx := context.WithValue(req.Context(), roleCtxKey{}, "viewer")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_NoRole(t *testing.T) {
	auth := NewAuthMiddleware(nil, "secret")
	handler := auth.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not pass without role in context")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_SameRole(t *testing.T) {
	auth := NewAuthMiddleware(nil, "test-secret")
	called := false
	handler := auth.RequireRole("operator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
	ctx := context.WithValue(req.Context(), roleCtxKey{}, "operator")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("operator should pass operator role check")
	}
}

func TestGetTenantID_Empty(t *testing.T) {
	if tid := GetTenantID(context.Background()); tid != "" {
		t.Fatalf("expected empty tenant, got %s", tid)
	}
}

func TestGetRole_Empty(t *testing.T) {
	if role := GetRole(context.Background()); role != "" {
		t.Fatalf("expected empty role, got %s", role)
	}
}

func TestGetEmail_Empty(t *testing.T) {
	if email := GetEmail(context.Background()); email != "" {
		t.Fatalf("expected empty email, got %s", email)
	}
}

func TestJWTClaims_Valid(t *testing.T) {
	claims := &JWTClaims{ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if err := claims.Valid(); err != nil {
		t.Fatalf("valid claims should pass: %v", err)
	}
}

func TestJWTClaims_Expired(t *testing.T) {
	claims := &JWTClaims{ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	if err := claims.Valid(); err == nil {
		t.Fatal("expired claims should fail Valid()")
	}
}

func TestParseJWT_WrongSigningMethod(t *testing.T) {
	claims := jwt.MapClaims{
		"tenant_id": "t1",
		"exp":       float64(time.Now().Add(time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := parseJWT(tokenString, []byte("any-secret"))
	if err == nil {
		t.Fatal("none signing method should be rejected")
	}
}

func TestParsePEMCertificate_Invalid(t *testing.T) {
	_, err := ParsePEMCertificate([]byte("not-a-pem"))
	if err == nil {
		t.Fatal("invalid PEM should fail")
	}
}

func TestParsePEMCertificate_Empty(t *testing.T) {
	_, err := ParsePEMCertificate([]byte(""))
	if err == nil {
		t.Fatal("empty PEM should fail")
	}
}

func TestSafeRedirect_Empty(t *testing.T) {
	if got := safeRedirect("", nil); got != "/" {
		t.Fatalf("expected /, got %s", got)
	}
}

func TestSafeRedirect_RelativePath(t *testing.T) {
	if got := safeRedirect("/dashboard", nil); got != "/dashboard" {
		t.Fatalf("expected /dashboard, got %s", got)
	}
}

func TestSafeRedirect_SlashOnly(t *testing.T) {
	if got := safeRedirect("/", nil); got != "/" {
		t.Fatalf("expected /, got %s", got)
	}
}

func TestSafeRedirect_DoubleSlash(t *testing.T) {
	if got := safeRedirect("//evil.com", nil); got != "/" {
		t.Fatalf("expected / for protocol-relative URL, got %s", got)
	}
}

func TestSafeRedirect_ExternalDisallowed(t *testing.T) {
	if got := safeRedirect("https://evil.com/phish", []string{" trusted.example.com"}); got != "/" {
		t.Fatalf("expected / for disallowed host, got %s", got)
	}
}

func TestSafeRedirect_ExternalAllowed(t *testing.T) {
	if got := safeRedirect("https://app.example.com/dashboard", []string{"app.example.com"}); got != "https://app.example.com/dashboard" {
		t.Fatalf("expected original URL for allowed host, got %s", got)
	}
}

func TestSafeRedirect_ExternalNoAllowedHosts(t *testing.T) {
	if got := safeRedirect("https://evil.com/phish", nil); got != "/" {
		t.Fatalf("expected / for external URL with no allowed hosts, got %s", got)
	}
}

func TestSafeRedirect_Backslash(t *testing.T) {
	if got := safeRedirect("\\evil.com", nil); got != "/" {
		t.Fatalf("expected / for non-absolute, non-slash path, got %s", got)
	}
}

func TestSAMLHandler_ParseSAMLResponse_NotImplemented(t *testing.T) {
	handler := NewSAMLHandler(nil, nil, "test-secret", nil)
	_, err := handler.parseSAMLResponse("dGVzdA==")
	if err == nil {
		t.Fatal("parseSAMLResponse should return error when SAML not implemented")
	}
}

func TestSAMLHandler_InitiateSSO_NotConfigured(t *testing.T) {
	handler := NewSAMLHandler(nil, nil, "test-secret", nil)
	req := httptest.NewRequest(http.MethodGet, "/sso?tenant_id=t1", nil)
	rec := httptest.NewRecorder()
	handler.InitiateSSO(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unconfigured SSO, got %d", rec.Code)
	}
}

func TestSAMLHandler_ACS_NotConfigured(t *testing.T) {
	handler := NewSAMLHandler(nil, nil, "test-secret", nil)
	req := httptest.NewRequest(http.MethodPost, "/sso/acs", strings.NewReader("SAMLResponse=dGVzdA=="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ACS(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for unconfigured SSO ACS, got %d", rec.Code)
	}
}

func TestSAMLHandler_ACS_NoCertificate(t *testing.T) {
	handler := NewSAMLHandler(&SAMLConfig{EntityID: "test", SsoURL: "https://sso.example.com"}, nil, "test-secret", nil)
	req := httptest.NewRequest(http.MethodPost, "/sso/acs", strings.NewReader("SAMLResponse=dGVzdA=="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ACS(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for SSO without certificate, got %d", rec.Code)
	}
}
