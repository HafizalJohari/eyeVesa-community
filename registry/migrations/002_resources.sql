-- AgentID Gateway: Resource Catalog
-- Stores enterprise resource endpoints and capabilities

CREATE TABLE resources (
    resource_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    endpoint VARCHAR(500) NOT NULL,
    auth_method VARCHAR(50) NOT NULL DEFAULT 'mTLS+SVID',
    capabilities JSONB DEFAULT '{}',
    risk_level VARCHAR(20) DEFAULT 'medium',
    data_sensitivity VARCHAR(50) DEFAULT 'internal',
    rate_limit_per_agent INTEGER DEFAULT 100,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_resources_type ON resources(resource_type);
CREATE INDEX idx_resources_risk_level ON resources(risk_level);
CREATE INDEX idx_resources_capabilities ON resources USING GIN(capabilities);

COMMENT ON TABLE resources IS 'Enterprise resource catalog exposing MCP capabilities';