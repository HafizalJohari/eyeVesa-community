-- eyeVesa: Multi-Tenant Isolation
-- Phase 3: Pro Feature (BSL 1.1 Licensed)

CREATE TABLE tenants (
    tenant_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    plan VARCHAR(20) DEFAULT 'community',
    max_agents INT DEFAULT 5,
    max_resources INT DEFAULT 10,
    sso_enabled BOOLEAN DEFAULT FALSE,
    sso_provider VARCHAR(50),
    sso_config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_plan ON tenants(plan);

-- Add tenant_id to agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id);

-- Add tenant_id to resources
ALTER TABLE resources ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(tenant_id);
CREATE INDEX IF NOT EXISTS idx_resources_tenant ON resources(tenant_id);

-- Add tenant_id to audit_logs
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant ON audit_logs(tenant_id);

-- Add tenant_id to hitl_approvals
ALTER TABLE hitl_approvals ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hitl_tenant ON hitl_approvals(tenant_id);

-- Approvers table: people who can approve HITL requests
CREATE TABLE approvers (
    approver_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(tenant_id),
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'approver',
    sso_subject VARCHAR(255),
    notification_channel VARCHAR(50) DEFAULT 'webhook',
    notification_target TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);

CREATE INDEX idx_approvers_tenant ON approvers(tenant_id);
CREATE INDEX idx_approvers_active ON approvers(tenant_id) WHERE is_active = TRUE;

COMMENT ON TABLE tenants IS 'Multi-tenant organizations with plan limits and SSO config';
COMMENT ON TABLE approvers IS 'Human approvers linked to tenants — identified by email or SSO subject';