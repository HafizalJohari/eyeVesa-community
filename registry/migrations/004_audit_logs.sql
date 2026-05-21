-- AgentID Gateway: Audit Evidence Vault
-- Non-repudiable, immutable audit trail for all machine actions

CREATE TABLE audit_logs (
    log_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    resource_id UUID REFERENCES resources(resource_id),
    action VARCHAR(255) NOT NULL,
    method VARCHAR(50) NOT NULL,
    params JSONB DEFAULT '{}',
    result JSONB DEFAULT '{}',
    result_status VARCHAR(20) NOT NULL,
    trust_score_before DECIMAL(5,4),
    trust_score_after DECIMAL(5,4),
    session_id UUID,
    ip_address INET,
    signature BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_agent ON audit_logs(agent_id);
CREATE INDEX idx_audit_resource ON audit_logs(resource_id);
CREATE INDEX idx_audit_created ON audit_logs(created_at);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_session ON audit_logs(session_id);

COMMENT ON TABLE audit_logs IS 'Immutable non-repudiable audit trail signed by gateway';
COMMENT ON COLUMN audit_logs.signature IS 'Cryptographic signature ensuring log integrity';