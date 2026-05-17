-- AgentID Gateway: SPIRE Trust Bundles & Federation
-- Stores trust bundles for cross-trust-domain verification

CREATE TABLE trust_bundles (
    bundle_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trust_domain VARCHAR(255) NOT NULL UNIQUE,
    bundle_data TEXT NOT NULL,
    bundle_type VARCHAR(20) NOT NULL DEFAULT 'spiffe_x509',
    source VARCHAR(50) NOT NULL DEFAULT 'static',
    endpoint_url VARCHAR(512),
    sequence_number BIGINT DEFAULT 1,
    expires_at TIMESTAMPTZ,
    is_federated BOOLEAN NOT NULL DEFAULT false,
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_trust_bundles_domain ON trust_bundles(trust_domain);
CREATE INDEX idx_trust_bundles_federated ON trust_bundles(is_federated);
CREATE INDEX idx_trust_bundles_expires ON trust_bundles(expires_at);

CREATE TABLE workload_registrations (
    registration_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    spiffe_id VARCHAR(512) NOT NULL UNIQUE,
    agent_id VARCHAR(255) NOT NULL,
    trust_domain VARCHAR(255) NOT NULL,
    selectors TEXT[] NOT NULL DEFAULT '{}',
    parent_id VARCHAR(512),
    auto_register BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attested_at TIMESTAMPTZ,
    registered_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_workload_reg_spiffe ON workload_registrations(spiffe_id);
CREATE INDEX idx_workload_reg_agent ON workload_registrations(agent_id);
CREATE INDEX idx_workload_reg_status ON workload_registrations(status);

COMMENT ON TABLE trust_bundles IS 'SPIRE trust bundles for federation and cross-domain verification';
COMMENT ON TABLE workload_registrations IS 'SPIRE workload attestation registrations linking SVIDs to agents';