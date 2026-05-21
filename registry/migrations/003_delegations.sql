-- AgentID Gateway: Delegation Chains
-- Tracks recursive agent-to-agent delegation with chain of custody

CREATE TABLE delegations (
    delegation_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_agent_id UUID NOT NULL REFERENCES agents(agent_id),
    child_agent_id UUID NOT NULL REFERENCES agents(agent_id),
    scope TEXT[] DEFAULT '{}',
    max_depth INTEGER DEFAULT 1,
    expires_at TIMESTAMPTZ NOT NULL,
    approved_by UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_delegations_parent ON delegations(parent_agent_id);
CREATE INDEX idx_delegations_child ON delegations(child_agent_id);
CREATE INDEX idx_delegations_expires ON delegations(expires_at);

COMMENT ON TABLE delegations IS 'Chain of custody for recursive agent-to-agent delegation';