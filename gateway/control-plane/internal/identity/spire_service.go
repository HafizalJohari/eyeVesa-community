package identity

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TrustBundle struct {
	BundleID       string `json:"bundle_id"`
	TrustDomain    string `json:"trust_domain"`
	BundleData     string `json:"bundle_data"`
	BundleType     string `json:"bundle_type"`
	Source         string `json:"source"`
	EndpointURL    string `json:"endpoint_url,omitempty"`
	SequenceNumber int64  `json:"sequence_number"`
	ExpiresAt      string `json:"expires_at,omitempty"`
	IsFederated    bool   `json:"is_federated"`
	Verified       bool   `json:"verified"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type WorkloadRegistration struct {
	RegistrationID string `json:"registration_id"`
	SpiffeID      string `json:"spiffe_id"`
	AgentID       string `json:"agent_id"`
	TrustDomain   string `json:"trust_domain"`
	Selectors     []string `json:"selectors"`
	ParentID      string `json:"parent_id,omitempty"`
	AutoRegister  bool   `json:"auto_register"`
	Status        string `json:"status"`
	AttestedAt    string `json:"attested_at,omitempty"`
	RegisteredAt  string `json:"registered_at"`
	ExpiresAt     string `json:"expires_at,omitempty"`
}

type SpireService struct {
	pool *pgxpool.Pool
}

func NewSpireService(pool *pgxpool.Pool) *SpireService {
	return &SpireService{pool: pool}
}

func (s *SpireService) RunBundleRefresh(ctx context.Context, interval time.Duration) {
	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Starting SPIRE bundle auto-refresh", "interval", interval)

	s.refreshAllWebBundles(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("SPIRE bundle auto-refresh stopped")
			return
		case <-ticker.C:
			s.refreshAllWebBundles(ctx)
		}
	}
}

func (s *SpireService) refreshAllWebBundles(ctx context.Context) {
	rows, err := s.pool.Query(ctx,
		`SELECT trust_domain, endpoint_url FROM trust_bundles WHERE source = 'web' AND endpoint_url IS NOT NULL AND endpoint_url != ''`)
	if err != nil {
		slog.Error("failed to query bundles for refresh", "error", err)
		return
	}
	defer rows.Close()

	type bundleRef struct {
		TrustDomain string
		EndpointURL string
	}
	var bundles []bundleRef

	for rows.Next() {
		var b bundleRef
		if err := rows.Scan(&b.TrustDomain, &b.EndpointURL); err != nil {
			slog.Error("failed to scan bundle for refresh", "error", err)
			continue
		}
		bundles = append(bundles, b)
	}

	if len(bundles) == 0 {
		return
	}

	slog.Info("Refreshing web-sourced bundles", "count", len(bundles))

	for _, b := range bundles {
		data, err := s.FetchBundleFromEndpoint(ctx, b.EndpointURL)
		if err != nil {
			slog.Error("failed to fetch bundle", "trust_domain", b.TrustDomain, "error", err)
			continue
		}

		if err := validateBundle(data); err != nil {
			slog.Error("bundle validation failed on refresh", "trust_domain", b.TrustDomain, "error", err)
			continue
		}

		_, updateErr := s.UpdateTrustBundle(ctx, b.TrustDomain, data)
		if updateErr != nil {
			slog.Error("failed to update refreshed bundle", "trust_domain", b.TrustDomain, "error", updateErr)
			continue
		}

		slog.Info("Refreshed trust bundle", "trust_domain", b.TrustDomain)
	}
}

func (s *SpireService) CreateTrustBundle(ctx context.Context, trustDomain, bundleData, bundleType, source, endpointURL string, isFederated bool) (*TrustBundle, error) {
	var b TrustBundle
	err := s.pool.QueryRow(ctx,
		`INSERT INTO trust_bundles (trust_domain, bundle_data, bundle_type, source, endpoint_url, is_federated, verified)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING bundle_id::text, trust_domain, bundle_data, bundle_type, source, endpoint_url, sequence_number,
		           COALESCE(expires_at::text, ''), is_federated, verified, created_at::text, updated_at::text`,
		trustDomain, bundleData, bundleType, source, endpointURL, isFederated, false,
	).Scan(&b.BundleID, &b.TrustDomain, &b.BundleData, &b.BundleType, &b.Source, &b.EndpointURL,
		&b.SequenceNumber, &b.ExpiresAt, &b.IsFederated, &b.Verified, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create trust bundle: %w", err)
	}
	return &b, nil
}

func (s *SpireService) GetTrustBundle(ctx context.Context, trustDomain string) (*TrustBundle, error) {
	var b TrustBundle
	err := s.pool.QueryRow(ctx,
		`SELECT bundle_id::text, trust_domain, bundle_data, bundle_type, source, endpoint_url, sequence_number,
		        COALESCE(expires_at::text, ''), is_federated, verified, created_at::text, updated_at::text
		 FROM trust_bundles WHERE trust_domain = $1`, trustDomain,
	).Scan(&b.BundleID, &b.TrustDomain, &b.BundleData, &b.BundleType, &b.Source, &b.EndpointURL,
		&b.SequenceNumber, &b.ExpiresAt, &b.IsFederated, &b.Verified, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get trust bundle: %w", err)
	}
	return &b, nil
}

func (s *SpireService) ListTrustBundles(ctx context.Context, federatedOnly bool) ([]TrustBundle, error) {
	query := `SELECT bundle_id::text, trust_domain, bundle_data, bundle_type, source, endpoint_url, sequence_number,
	         COALESCE(expires_at::text, ''), is_federated, verified, created_at::text, updated_at::text
	         FROM trust_bundles`
	if federatedOnly {
		query += " WHERE is_federated = true"
	}
	query += " ORDER BY trust_domain"

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list trust bundles: %w", err)
	}
	defer rows.Close()

	var bundles []TrustBundle
	for rows.Next() {
		var b TrustBundle
		if err := rows.Scan(&b.BundleID, &b.TrustDomain, &b.BundleData, &b.BundleType, &b.Source,
			&b.EndpointURL, &b.SequenceNumber, &b.ExpiresAt, &b.IsFederated, &b.Verified,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan trust bundle: %w", err)
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

func (s *SpireService) UpdateTrustBundle(ctx context.Context, trustDomain, bundleData string) (*TrustBundle, error) {
	var b TrustBundle
	err := s.pool.QueryRow(ctx,
		`UPDATE trust_bundles SET bundle_data = $2, sequence_number = sequence_number + 1,
		        verified = false, updated_at = NOW()
		 WHERE trust_domain = $1
		 RETURNING bundle_id::text, trust_domain, bundle_data, bundle_type, source, endpoint_url, sequence_number,
		           COALESCE(expires_at::text, ''), is_federated, verified, created_at::text, updated_at::text`,
		trustDomain, bundleData,
	).Scan(&b.BundleID, &b.TrustDomain, &b.BundleData, &b.BundleType, &b.Source, &b.EndpointURL,
		&b.SequenceNumber, &b.ExpiresAt, &b.IsFederated, &b.Verified, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update trust bundle: %w", err)
	}
	return &b, nil
}

func (s *SpireService) VerifyTrustBundle(ctx context.Context, trustDomain string) error {
	bundle, err := s.GetTrustBundle(ctx, trustDomain)
	if err != nil {
		return err
	}

	if err := validateBundle(bundle.BundleData); err != nil {
		return fmt.Errorf("bundle validation failed: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE trust_bundles SET verified = true, updated_at = NOW() WHERE trust_domain = $1`,
		trustDomain)
	if err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}

	slog.Info("Trust bundle verified", "trust_domain", trustDomain)
	return nil
}

func (s *SpireService) DeleteTrustBundle(ctx context.Context, trustDomain string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM trust_bundles WHERE trust_domain = $1`, trustDomain)
	if err != nil {
		return fmt.Errorf("delete trust bundle: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("trust bundle not found: %s", trustDomain)
	}
	return nil
}

func (s *SpireService) RegisterWorkload(ctx context.Context, spiffeID, agentID, trustDomain string, selectors []string, parentID string, autoRegister bool) (*WorkloadRegistration, error) {
	selectorsJSON, _ := json.Marshal(selectors)
	var w WorkloadRegistration
	err := s.pool.QueryRow(ctx,
		`INSERT INTO workload_registrations (spiffe_id, agent_id, trust_domain, selectors, parent_id, auto_register, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active')
		 RETURNING registration_id::text, spiffe_id, agent_id, trust_domain, selectors, parent_id, auto_register, status,
		           COALESCE(attested_at::text, ''), registered_at::text, COALESCE(expires_at::text, '')`,
		spiffeID, agentID, trustDomain, string(selectorsJSON), parentID, autoRegister,
	).Scan(&w.RegistrationID, &w.SpiffeID, &w.AgentID, &w.TrustDomain, &w.Selectors, &w.ParentID,
		&w.AutoRegister, &w.Status, &w.AttestedAt, &w.RegisteredAt, &w.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("register workload: %w", err)
	}

	if selStr, ok := parseSelectors(w.Selectors); ok {
		w.Selectors = selStr
	}

	return &w, nil
}

func (s *SpireService) GetWorkload(ctx context.Context, spiffeID string) (*WorkloadRegistration, error) {
	var w WorkloadRegistration
	err := s.pool.QueryRow(ctx,
		`SELECT registration_id::text, spiffe_id, agent_id, trust_domain, selectors, parent_id, auto_register, status,
		        COALESCE(attested_at::text, ''), registered_at::text, COALESCE(expires_at::text, '')
		 FROM workload_registrations WHERE spiffe_id = $1`, spiffeID,
	).Scan(&w.RegistrationID, &w.SpiffeID, &w.AgentID, &w.TrustDomain, &w.Selectors,
		&w.ParentID, &w.AutoRegister, &w.Status, &w.AttestedAt, &w.RegisteredAt, &w.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("get workload: %w", err)
	}
	return &w, nil
}

func (s *SpireService) ListWorkloads(ctx context.Context, agentID string) ([]WorkloadRegistration, error) {
	query := `SELECT registration_id::text, spiffe_id, agent_id, trust_domain, selectors, parent_id, auto_register, status,
	          COALESCE(attested_at::text, ''), registered_at::text, COALESCE(expires_at::text, '')
	          FROM workload_registrations`
	args := []interface{}{}
	if agentID != "" {
		query += " WHERE agent_id = $1"
		args = append(args, agentID)
	}
	query += " ORDER BY registered_at DESC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workloads: %w", err)
	}
	defer rows.Close()

	var workloads []WorkloadRegistration
	for rows.Next() {
		var w WorkloadRegistration
		if err := rows.Scan(&w.RegistrationID, &w.SpiffeID, &w.AgentID, &w.TrustDomain,
			&w.Selectors, &w.ParentID, &w.AutoRegister, &w.Status,
			&w.AttestedAt, &w.RegisteredAt, &w.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan workload: %w", err)
		}
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (s *SpireService) AttestWorkload(ctx context.Context, spiffeID string) (*WorkloadRegistration, error) {
	var w WorkloadRegistration
	err := s.pool.QueryRow(ctx,
		`UPDATE workload_registrations SET attested_at = NOW(), status = 'attested'
		 WHERE spiffe_id = $1
		 RETURNING registration_id::text, spiffe_id, agent_id, trust_domain, selectors, parent_id, auto_register, status,
		           COALESCE(attested_at::text, ''), registered_at::text, COALESCE(expires_at::text, '')`,
		spiffeID,
	).Scan(&w.RegistrationID, &w.SpiffeID, &w.AgentID, &w.TrustDomain, &w.Selectors,
		&w.ParentID, &w.AutoRegister, &w.Status, &w.AttestedAt, &w.RegisteredAt, &w.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("attest workload: %w", err)
	}
	return &w, nil
}

func (s *SpireService) DeleteWorkload(ctx context.Context, spiffeID string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM workload_registrations WHERE spiffe_id = $1`, spiffeID)
	if err != nil {
		return fmt.Errorf("delete workload: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("workload not found: %s", spiffeID)
	}
	return nil
}

type SpireStatus struct {
	Available       bool   `json:"available"`
	SocketPath      string `json:"socket_path"`
	TrustDomain     string `json:"trust_domain"`
	OwnSpiffeID     string `json:"own_spiffe_id"`
	SVIDExpiry      string `json:"svid_expiry"`
	BundleCount     int    `json:"bundle_count"`
	FederatedCount  int    `json:"federated_count"`
	WorkloadCount   int    `json:"workload_count"`
	AttestedCount   int    `json:"attested_count"`
}

func (s *SpireService) GetStatus(ctx context.Context, provider IdentityProvider) (*SpireStatus, error) {
	status := &SpireStatus{}

	svid, err := provider.FetchSVID(ctx)
	if err == nil && svid != nil {
		status.Available = true
		status.TrustDomain = svid.TrustDomain
		status.OwnSpiffeID = svid.SpiffeID
		status.SVIDExpiry = svid.ExpiresAt.Format(time.RFC3339)
	} else {
		status.Available = false
	}

	socketPath := os.Getenv("SPIRE_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}
	status.SocketPath = socketPath

	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trust_bundles`).Scan(&status.BundleCount)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trust_bundles WHERE is_federated = true`).Scan(&status.FederatedCount)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workload_registrations`).Scan(&status.WorkloadCount)
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workload_registrations WHERE status = 'attested'`).Scan(&status.AttestedCount)

	return status, nil
}

func (s *SpireService) FetchBundleFromEndpoint(ctx context.Context, endpointURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpointURL)
	if err != nil {
		return "", fmt.Errorf("fetch bundle from %s: %w", endpointURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bundle endpoint returned %d", resp.StatusCode)
	}

	var bundleResp struct {
		Keys       json.RawMessage `json:"keys"`
		SequenceNumber int64          `json:"sequence_number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundleResp); err != nil {
		return "", fmt.Errorf("decode bundle: %w", err)
	}

	return string(bundleResp.Keys), nil
}

func validateBundle(bundleData string) error {
	data := strings.TrimSpace(bundleData)

	if data == "" {
		return fmt.Errorf("bundle data is empty")
	}

	if strings.HasPrefix(data, "{") {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return fmt.Errorf("invalid JWKSet JSON: %w", err)
		}
		keys, ok := parsed["keys"]
		if !ok || keys == nil {
			return fmt.Errorf("JWKSet missing 'keys' field")
		}
		return nil
	}

	block, _ := pem.Decode([]byte(data))
	if block != nil {
		certs, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			return fmt.Errorf("invalid PEM certificate: %w", err)
		}
		if len(certs) == 0 {
			return fmt.Errorf("no certificates found in PEM data")
		}
		return nil
	}

	asn1Data := []byte(data)
	if _, err := x509.ParseCertificates(asn1Data); err != nil {
		return fmt.Errorf("invalid bundle format (must be JWKSet JSON, PEM, or DER)")
	}

	return nil
}

func parseSelectors(v interface{}) ([]string, bool) {
	switch s := v.(type) {
	case string:
		if s == "" {
			return nil, true
		}
		if s == "[]" {
			return []string{}, true
		}
		var result []string
		if err := json.Unmarshal([]byte(s), &result); err == nil {
			return result, true
		}
		return []string{s}, true
	case []string:
		return s, true
	case []interface{}:
		result := make([]string, len(s))
		for i, item := range s {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result, true
	default:
		return nil, false
	}
}