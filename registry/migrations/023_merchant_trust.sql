-- Merchant role and trust engine (hybrid objective + feedback)

ALTER TABLE agents
ADD COLUMN IF NOT EXISTS roles TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_agents_roles ON agents USING GIN(roles);

CREATE TABLE IF NOT EXISTS merchant_profiles (
    merchant_id UUID PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    business_type VARCHAR(64) NOT NULL DEFAULT 'digital_goods',
    categories TEXT[] NOT NULL DEFAULT '{}',
    fulfillment_model VARCHAR(64) NOT NULL DEFAULT 'api',
    regions TEXT[] NOT NULL DEFAULT '{}',
    support_sla VARCHAR(64) NOT NULL DEFAULT 'best_effort',
    verification_tier VARCHAR(32) NOT NULL DEFAULT 'unverified',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS merchant_trust_state (
    merchant_id UUID PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    trust_score DECIMAL(5,4) NOT NULL DEFAULT 0.5000 CHECK (trust_score BETWEEN 0 AND 1),
    confidence DECIMAL(5,4) NOT NULL DEFAULT 0.1000 CHECK (confidence BETWEEN 0 AND 1),
    volume_bucket VARCHAR(32) NOT NULL DEFAULT 'low',
    risk_flags TEXT[] NOT NULL DEFAULT '{}',
    total_objective_events INTEGER NOT NULL DEFAULT 0,
    total_feedback_events INTEGER NOT NULL DEFAULT 0,
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    hitl_only BOOLEAN NOT NULL DEFAULT FALSE,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_merchant_trust_score ON merchant_trust_state(trust_score DESC);

CREATE TABLE IF NOT EXISTS merchant_trust_events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    buyer_agent_id UUID REFERENCES agents(agent_id) ON DELETE SET NULL,
    event_kind VARCHAR(16) NOT NULL CHECK (event_kind IN ('objective', 'feedback')),
    outcome_type VARCHAR(32),
    stars SMALLINT,
    sentiment_score DECIMAL(4,3),
    complaint_severity SMALLINT,
    order_id VARCHAR(128),
    dispute_ref VARCHAR(128),
    receipt_signature TEXT,
    event_ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_merchant_events_merchant_ts ON merchant_trust_events(merchant_id, event_ts DESC);
CREATE INDEX IF NOT EXISTS idx_merchant_events_order_id ON merchant_trust_events(order_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_merchant_objective_order_outcome
ON merchant_trust_events(merchant_id, order_id, outcome_type)
WHERE event_kind = 'objective' AND order_id IS NOT NULL;

COMMENT ON TABLE merchant_profiles IS 'Merchant role metadata linked to a base agent identity';
COMMENT ON TABLE merchant_trust_state IS 'Current merchant trust and confidence state';
COMMENT ON TABLE merchant_trust_events IS 'Objective and feedback events used by the hybrid trust engine';
