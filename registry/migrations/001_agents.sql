-- AgentID Gateway: Agent Registry
-- Stores AI agent identities and metadata

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE agents (
    agent_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    owner VARCHAR(255) NOT NULL,
    public_key BYTEA NOT NULL,
    capabilities TEXT[] DEFAULT '{}',
    allowed_tools TEXT[] DEFAULT '{}',
    max_budget_usd DECIMAL(10,2) DEFAULT 0.00,
    delegation_policy VARCHAR(50) DEFAULT 'no_chain',
    behavioral_tags TEXT[] DEFAULT '{}',
    trust_score DECIMAL(5,4) DEFAULT 1.0000,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agents_owner ON agents(owner);
CREATE INDEX idx_agents_trust_score ON agents(trust_score);
CREATE INDEX idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX idx_agents_behavioral_tags ON agents USING GIN(behavioral_tags);

COMMENT ON TABLE agents IS 'AI agent identity registry with trust scores';

-- pgvector column: uncomment when pgvector extension is available
-- CREATE EXTENSION IF NOT EXISTS "pgvector";
-- ALTER TABLE agents ADD COLUMN behavior_vec vector(1536);
-- CREATE INDEX idx_agents_behavior_vec ON agents USING ivfflat (behavior_vec vector_cosine_ops) WITH (lists = 100);