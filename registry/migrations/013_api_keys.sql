-- eyeVesa: API Keys for authentication
-- Phase 3: Auth middleware

CREATE TABLE api_keys (
    key_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(tenant_id),
    api_key VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_key ON api_keys(api_key) WHERE is_active = TRUE;
CREATE INDEX idx_api_keys_tenant ON api_keys(tenant_id);

COMMENT ON TABLE api_keys IS 'API keys for authenticating requests to the gateway';