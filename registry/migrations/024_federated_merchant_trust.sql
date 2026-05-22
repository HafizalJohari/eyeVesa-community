-- Federated merchant trust sync store (International Airport)

CREATE TABLE IF NOT EXISTS federated_merchant_trust (
    merchant_id UUID PRIMARY KEY REFERENCES federated_agents(agent_id) ON DELETE CASCADE,
    gateway_id UUID NOT NULL REFERENCES federation_peers(gateway_id) ON DELETE CASCADE,
    trust_score FLOAT NOT NULL DEFAULT 0.5,
    confidence FLOAT NOT NULL DEFAULT 0.1,
    verification_tier TEXT NOT NULL DEFAULT 'unverified',
    risk_flags TEXT[] NOT NULL DEFAULT '{}',
    hitl_only BOOLEAN NOT NULL DEFAULT FALSE,
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    order_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS federated_merchant_trust_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES federated_agents(agent_id) ON DELETE CASCADE,
    gateway_id UUID NOT NULL REFERENCES federation_peers(gateway_id) ON DELETE CASCADE,
    order_id TEXT NOT NULL,
    outcome_status TEXT NOT NULL DEFAULT '',
    dispute_ref TEXT NOT NULL DEFAULT '',
    receipt_signature TEXT NOT NULL DEFAULT '',
    event_ts TIMESTAMP NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_fed_merchant_event_dedupe
ON federated_merchant_trust_events(merchant_id, gateway_id, order_id, outcome_status, receipt_signature);

CREATE INDEX IF NOT EXISTS idx_fed_merchant_trust_score
ON federated_merchant_trust(trust_score DESC);
