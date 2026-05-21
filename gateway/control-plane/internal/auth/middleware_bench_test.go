package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkMiddlewarePublicPath(b *testing.B) {
	auth := NewAuthMiddleware(nil, "bench-secret")
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMiddlewareBearerToken(b *testing.B) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := buildJWTToken(&JWTClaims{
		TenantID:  "bench",
		Role:     "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkMiddlewareUnauthorized(b *testing.B) {
	auth := NewAuthMiddleware(nil, "bench-secret")
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkParseJWTViaHMAC(b *testing.B) {
	secret := []byte("bench-jwt-secret-for-load-testing-32b")
	token := buildJWTToken(&JWTClaims{
		TenantID:  "bench",
		Role:     "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseJWT(token, secret)
	}
}

func BenchmarkBuildJWTToken(b *testing.B) {
	secret := []byte("bench-jwt-secret-for-load-testing-32b")
	claims := &JWTClaims{
		TenantID:  "bench",
		Role:     "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildJWTToken(claims, secret)
	}
}

func BenchmarkGenerateAPIKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateAPIKey()
	}
}

func BenchmarkRequireRole(b *testing.B) {
	secret := string(GenerateJWTSecret())
	auth := NewAuthMiddleware(nil, secret)
	handler := auth.RequireRole("operator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := buildJWTToken(&JWTClaims{
		Role:      "admin",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, []byte(secret))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}