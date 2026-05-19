-- eyeVesa Federation: Gateway registry + federated agent passports
-- Migration 018: Multi-gateway federation for Central Airport

-- Federated gateways (embassy registry)
-- Local gateways register here so their passports are accepted
CREATE TABLE IF NOT EXISTS federation_peers (
    gateway_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    public_key BYTEA NOT NULL,
    endpoint TEXT NOT NULL UNIQUE,
    trust_domain TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    trust_score FLOAT NOT NULL DEFAULT 1.0,
    agent_count INT NOT NULL DEFAULT 0,
    last_sync_at TIMESTAMP DEFAULT NULL,
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_federation_peers_status ON federation_peers(status);
CREATE INDEX idx_federation_peers_endpoint ON federation_peers(endpoint);

-- Federated agent cache (passports from remote gateways)
-- When a local gateway syncs an agent to Central Airport, we store the passport here
CREATE TABLE IF NOT EXISTS federated_agents (
    agent_id UUID PRIMARY KEY,
    gateway_id UUID NOT NULL REFERENCES federation_peers(gateway_id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    public_key BYTEA NOT NULL,
    trust_score FLOAT NOT NULL DEFAULT 1.0,
    capabilities TEXT[] DEFAULT '{}',
    allowed_tools TEXT[] DEFAULT '{}',
    passport_signature BYTEA NOT NULL,
    passport_issued_at TIMESTAMP NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_federated_agents_gateway ON federated_agents(gateway_id);
CREATE INDEX idx_federated_agents_status ON federated_agents(status);
CREATE INDEX idx_federated_agents_owner ON federated_agents(owner);

-- Federated airport profiles (mirrors agent_profiles for remote agents)
CREATE TABLE IF NOT EXISTS federated_profiles (
    agent_id UUID PRIMARY KEY REFERENCES federated_agents(agent_id) ON DELETE CASCADE,
    description TEXT DEFAULT '',
    services_offered JSONB DEFAULT '[]',
    endpoints JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    listed BOOLEAN DEFAULT true,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_federated_profiles_listed ON federated_profiles(listed) WHERE listed = true;

-- Federated heartbeats (remote agents announce presence at Central Airport)
CREATE TABLE IF NOT EXISTS federated_heartbeats (
    agent_id UUID PRIMARY KEY REFERENCES federated_agents(agent_id) ON DELETE CASCADE,
    gateway_id UUID NOT NULL REFERENCES federation_peers(gateway_id) ON DELETE CASCADE,
    last_heartbeat TIMESTAMP NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline', 'busy', 'idle')),
    metadata JSONB DEFAULT '{}',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_federated_heartbeats_status ON federated_heartbeats(status);
CREATE INDEX idx_federated_heartbeats_last ON federated_heartbeats(last_heartbeat DESC);

-- Federated connections (cross-gateway interaction log)
CREATE TABLE IF NOT EXISTS federated_connections (
    connection_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL,
    responder_id UUID NOT NULL,
    requester_gateway_id UUID REFERENCES federation_peers(gateway_id),
    responder_gateway_id UUID REFERENCES federation_peers(gateway_id),
    action TEXT NOT NULL,
    outcome TEXT NOT NULL DEFAULT 'success' CHECK (outcome IN ('success', 'denied', 'hitl_required', 'timeout', 'error')),
    trust_score_at_time FLOAT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fed_conn_requester ON federated_connections(requester_id);
CREATE INDEX idx_fed_conn_responder ON federated_connections(responder_id);
CREATE INDEX idx_fed_conn_created ON federated_connections(created_at DESC);

-- Function: mark federated agents offline if heartbeat older than 5 minutes
CREATE OR REPLACE FUNCTION federated_mark_stale_offline()
RETURNS void AS $$
BEGIN
    UPDATE federated_heartbeats
    SET status = 'offline', updated_at = NOW()
    WHERE last_heartbeat < NOW() - INTERVAL '5 minutes'
      AND status != 'offline';
END;
$$ LANGUAGE plpgsql;