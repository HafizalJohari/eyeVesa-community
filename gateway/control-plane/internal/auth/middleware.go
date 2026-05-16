package auth

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthMiddleware struct {
	db        *pgxpool.Pool
	apiKeys   map[string]string
	jwtSecret []byte
}

func NewAuthMiddleware(db *pgxpool.Pool, jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		db:        db,
		apiKeys:   make(map[string]string),
		jwtSecret: []byte(jwtSecret),
	}
}

func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if a.checkAPIKey(r) {
			next.ServeHTTP(w, r)
			return
		}

		if a.checkBearerToken(r) {
			next.ServeHTTP(w, r)
			return
		}

		if tenantID, ok := a.checkSSOToken(r); ok {
			ctx := context.WithValue(r.Context(), tenantCtxKey{}, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized","message":"valid API key, bearer token, or SSO session required"}`))
	})
}

func isPublicPath(path string) bool {
	public := []string{"/health", "/identity"}
	for _, p := range public {
		if path == p {
			return true
		}
	}
	if strings.HasPrefix(path, "/v1/agents/register") ||
		strings.HasPrefix(path, "/v1/resources/register") ||
		strings.HasPrefix(path, "/v1/mcp") {
		return true
	}
	return false
}

func (a *AuthMiddleware) checkAPIKey(r *http.Request) bool {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return false
	}

	var apiKey, tenantID string
	err := a.db.QueryRow(r.Context(),
		`SELECT api_key, tenant_id FROM api_keys WHERE api_key = $1 AND is_active = TRUE`,
		key,
	).Scan(&apiKey, &tenantID)
	if err != nil {
		return false
	}

	return true
}

func (a *AuthMiddleware) checkBearerToken(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return false
	}

	claims, err := parseJWT(token, a.jwtSecret)
	if err != nil {
		return false
	}

	if claims.ExpiresAt < time.Now().Unix() {
		return false
	}

	return true
}

func (a *AuthMiddleware) checkSSOToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("eyevesa_sso")
	if err != nil {
		return "", false
	}

	claims, err := parseJWT(cookie.Value, a.jwtSecret)
	if err != nil {
		return "", false
	}

	if claims.ExpiresAt < time.Now().Unix() {
		return "", false
	}

	if claims.TenantID == "" {
		return "", false
	}

	return claims.TenantID, true
}

type JWTClaims struct {
	TenantID  string `json:"tenant_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

func (c *JWTClaims) Valid() error {
	if time.Now().Unix() > c.ExpiresAt {
		return fmt.Errorf("token expired")
	}
	return nil
}

func parseJWT(tokenString string, secret []byte) (*JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	c := &JWTClaims{}
	if v, ok := claims["tenant_id"].(string); ok {
		c.TenantID = v
	}
	if v, ok := claims["email"].(string); ok {
		c.Email = v
	}
	if v, ok := claims["role"].(string); ok {
		c.Role = v
	}
	if v, ok := claims["exp"].(float64); ok {
		c.ExpiresAt = int64(v)
	}
	if v, ok := claims["iat"].(float64); ok {
		c.IssuedAt = int64(v)
	}

	return c, nil
}

type tenantCtxKey struct{}

func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(tenantCtxKey{}).(string); ok {
		return v
	}
	return ""
}

func (a *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden","message":"insufficient role"}`))
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			claims, err := parseJWT(token, a.jwtSecret)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden","message":"insufficient role"}`))
				return
			}

			roleOrder := map[string]int{"admin": 3, "operator": 2, "viewer": 1}
			if roleOrder[claims.Role] < roleOrder[role] {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"forbidden","message":"insufficient role"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type SAMLConfig struct {
	EntityID    string
	SsoURL      string
	SloURL      string
	Certificate *x509.Certificate
	PrivateKey  interface{}
}

type SAMLHandler struct {
	config *SAMLConfig
	db     *pgxpool.Pool
	secret []byte
}

func NewSAMLHandler(config *SAMLConfig, db *pgxpool.Pool, jwtSecret string) *SAMLHandler {
	return &SAMLHandler{
		config: config,
		db:     db,
		secret: []byte(jwtSecret),
	}
}

func (h *SAMLHandler) InitiateSSO(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}

	authURL := fmt.Sprintf("%s?SAMLRequest=%s&RelayState=%s",
		h.config.SsoURL,
		base64.URLEncoding.EncodeToString([]byte(buildSAMLRequest(h.config, tenantID))),
		tenantID,
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *SAMLHandler) ACS(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid SAML response", http.StatusBadRequest)
		return
	}

	samlResponse := r.FormValue("SAMLResponse")
	relayState := r.FormValue("RelayState")

	claims, err := h.parseSAMLResponse(samlResponse)
	if err != nil {
		http.Error(w, "SAML validation failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	claims.TenantID = relayState
	token := buildJWTToken(claims, h.secret)

	http.SetCookie(w, &http.Cookie{
		Name:     "eyevesa_sso",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *SAMLHandler) parseSAMLResponse(encoded string) (*JWTClaims, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode SAML: %w", err)
	}

	_ = data // Production: use github.com/crewjam/saml for actual SAML parsing

	return &JWTClaims{
		Role:      "approver",
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}, nil
}

func buildSAMLRequest(config *SAMLConfig, tenantID string) []byte {
	return []byte(fmt.Sprintf(
		`<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="%s" Version="2.0" IssueInstant="%s" Destination="%s"><saml:Issuer xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">%s</saml:Issuer></samlp:AuthnRequest>`,
		"eyevesa-"+tenantID,
		time.Now().Format(time.RFC3339),
		config.SsoURL,
		config.EntityID,
	))
}

func buildJWTToken(claims *JWTClaims, secret []byte) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": claims.TenantID,
		"email":     claims.Email,
		"role":      claims.Role,
		"exp":       claims.ExpiresAt,
		"iat":       claims.IssuedAt,
	})

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return ""
	}
	return tokenString
}

func GenerateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "eyevesa_" + base64.RawURLEncoding.EncodeToString(b)
}

func GenerateJWTSecret() []byte {
	b := make([]byte, 64)
	rand.Read(b)
	return []byte(base64.RawURLEncoding.EncodeToString(b))
}

func ParsePEMCertificate(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}