-- AgentID Gateway: Identity Bindings (PTV Protocol)
-- Binds agent identities to hardware roots of trust

CREATE TABLE identity_bindings (
    binding_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    platform VARCHAR(100) NOT NULL,
    runtime_hash BYTEA NOT NULL,
    hardware_public_key BYTEA NOT NULL,
    binding_signature BYTEA NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_identity_bindings_agent ON identity_bindings(agent_id);
CREATE INDEX idx_identity_bindings_status ON identity_bindings(status);
CREATE INDEX idx_identity_bindings_expires ON identity_bindings(expires_at);

COMMENT ON TABLE identity_bindings IS 'PTV identity bindings - cryptographic proof that an agent runs on specific hardware';