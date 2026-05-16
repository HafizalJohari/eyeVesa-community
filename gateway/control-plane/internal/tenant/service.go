package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tenant struct {
	TenantID   string    `json:"tenant_id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	Plan       string    `json:"plan"`
	MaxAgents  int       `json:"max_agents"`
	MaxResources int     `json:"max_resources"`
	SSOEnabled bool      `json:"sso_enabled"`
	SSOProvider string   `json:"sso_provider,omitempty"`
	SSOConfig  string    `json:"sso_config,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TenantService struct {
	db *pgxpool.Pool
}

func NewTenantService(db *pgxpool.Pool) *TenantService {
	return &TenantService{db: db}
}

func (s *TenantService) CreateTenant(ctx context.Context, name, slug, plan string, maxAgents, maxResources int) (*Tenant, error) {
	id := uuid.New()
	_, err := s.db.Exec(ctx,
		`INSERT INTO tenants (tenant_id, name, slug, plan, max_agents, max_resources)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, name, slug, plan, maxAgents, maxResources,
	)
	if err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	return &Tenant{
		TenantID:     id.String(),
		Name:         name,
		Slug:         slug,
		Plan:         plan,
		MaxAgents:    maxAgents,
		MaxResources: maxResources,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (s *TenantService) GetTenant(ctx context.Context, tenantID string) (*Tenant, error) {
	var t Tenant
	err := s.db.QueryRow(ctx,
		`SELECT tenant_id, name, slug, plan, max_agents, max_resources, sso_enabled, COALESCE(sso_provider, ''), COALESCE(sso_config::text, ''), created_at, updated_at
		 FROM tenants WHERE tenant_id = $1`,
		tenantID,
	).Scan(&t.TenantID, &t.Name, &t.Slug, &t.Plan, &t.MaxAgents, &t.MaxResources, &t.SSOEnabled, &t.SSOProvider, &t.SSOConfig, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	return &t, nil
}

func (s *TenantService) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var t Tenant
	err := s.db.QueryRow(ctx,
		`SELECT tenant_id, name, slug, plan, max_agents, max_resources, sso_enabled, COALESCE(sso_provider, ''), COALESCE(sso_config::text, ''), created_at, updated_at
		 FROM tenants WHERE slug = $1`,
		slug,
	).Scan(&t.TenantID, &t.Name, &t.Slug, &t.Plan, &t.MaxAgents, &t.MaxResources, &t.SSOEnabled, &t.SSOProvider, &t.SSOConfig, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	return &t, nil
}

func (s *TenantService) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.db.Query(ctx,
		`SELECT tenant_id, name, slug, plan, max_agents, max_resources, sso_enabled, COALESCE(sso_provider, ''), COALESCE(sso_config::text, ''), created_at, updated_at
		 FROM tenants ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.TenantID, &t.Name, &t.Slug, &t.Plan, &t.MaxAgents, &t.MaxResources, &t.SSOEnabled, &t.SSOProvider, &t.SSOConfig, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (s *TenantService) UpdatePlan(ctx context.Context, tenantID, plan string, maxAgents, maxResources int) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenants SET plan = $1, max_agents = $2, max_resources = $3, updated_at = NOW() WHERE tenant_id = $4`,
		plan, maxAgents, maxResources, tenantID,
	)
	return err
}

func (s *TenantService) EnableSSO(ctx context.Context, tenantID, provider, config string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenants SET sso_enabled = TRUE, sso_provider = $1, sso_config = $2::jsonb, updated_at = NOW() WHERE tenant_id = $3`,
		provider, config, tenantID,
	)
	return err
}

func (s *TenantService) CheckAgentLimit(ctx context.Context, tenantID string) (bool, int, int, error) {
	var maxAgents int
	var current int
	err := s.db.QueryRow(ctx,
		`SELECT t.max_agents, COUNT(a.agent_id) FROM tenants t LEFT JOIN agents a ON a.tenant_id = t.tenant_id WHERE t.tenant_id = $1 GROUP BY t.max_agents`,
		tenantID,
	).Scan(&maxAgents, &current)
	if err != nil {
		return true, 0, 0, nil
	}
	return current < maxAgents, current, maxAgents, nil
}

func (s *TenantService) CheckResourceLimit(ctx context.Context, tenantID string) (bool, int, int, error) {
	var maxResources int
	var current int
	err := s.db.QueryRow(ctx,
		`SELECT t.max_resources, COUNT(r.resource_id) FROM tenants t LEFT JOIN resources r ON r.tenant_id = t.tenant_id WHERE t.tenant_id = $1 GROUP BY t.max_resources`,
		tenantID,
	).Scan(&maxResources, &current)
	if err != nil {
		return true, 0, 0, nil
	}
	return current < maxResources, current, maxResources, nil
}

type Approver struct {
	ApproverID        string    `json:"approver_id"`
	TenantID          string    `json:"tenant_id"`
	Email             string    `json:"email"`
	Name              string    `json:"name"`
	Role              string    `json:"role"`
	SSOSubject        string    `json:"sso_subject,omitempty"`
	NotificationChannel string  `json:"notification_channel,omitempty"`
	NotificationTarget string   `json:"notification_target,omitempty"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *TenantService) CreateApprover(ctx context.Context, tenantID, email, name, role string) (*Approver, error) {
	id := uuid.New()
	_, err := s.db.Exec(ctx,
		`INSERT INTO approvers (approver_id, tenant_id, email, name, role) VALUES ($1, $2, $3, $4, $5)`,
		id, tenantID, email, name, role,
	)
	if err != nil {
		return nil, fmt.Errorf("create approver: %w", err)
	}
	return &Approver{
		ApproverID: id.String(),
		TenantID:   tenantID,
		Email:      email,
		Name:       name,
		Role:       role,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}, nil
}

func (s *TenantService) ListApprovers(ctx context.Context, tenantID string) ([]Approver, error) {
	rows, err := s.db.Query(ctx,
		`SELECT approver_id, tenant_id, email, name, role, COALESCE(sso_subject, ''), notification_channel, COALESCE(notification_target, ''), is_active, created_at
		 FROM approvers WHERE tenant_id = $1 AND is_active = TRUE ORDER BY name`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var approvers []Approver
	for rows.Next() {
		var a Approver
		if err := rows.Scan(&a.ApproverID, &a.TenantID, &a.Email, &a.Name, &a.Role, &a.SSOSubject, &a.NotificationChannel, &a.NotificationTarget, &a.IsActive, &a.CreatedAt); err != nil {
			continue
		}
		approvers = append(approvers, a)
	}
	return approvers, nil
}

func (s *TenantService) FindApproverBySSO(ctx context.Context, tenantID, ssoSubject string) (*Approver, error) {
	var a Approver
	err := s.db.QueryRow(ctx,
		`SELECT approver_id, tenant_id, email, name, role, COALESCE(sso_subject, ''), notification_channel, COALESCE(notification_target, ''), is_active
		 FROM approvers WHERE tenant_id = $1 AND sso_subject = $2 AND is_active = TRUE`,
		tenantID, ssoSubject,
	).Scan(&a.ApproverID, &a.TenantID, &a.Email, &a.Name, &a.Role, &a.SSOSubject, &a.NotificationChannel, &a.NotificationTarget, &a.IsActive)
	if err != nil {
		return nil, fmt.Errorf("approver not found: %w", err)
	}
	return &a, nil
}

func (s *TenantService) DeactivateApprover(ctx context.Context, approverID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE approvers SET is_active = FALSE, updated_at = NOW() WHERE approver_id = $1`,
		approverID,
	)
	return err
}