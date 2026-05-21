-- AgentID Gateway: Trust & Behavior Tracking
-- Session-aware trust degradation and behavioral anomaly detection

CREATE TABLE trust_events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    event_type VARCHAR(50) NOT NULL,
    trust_delta DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    trust_score_after DECIMAL(5,4) NOT NULL,
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    session_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_trust_events_agent ON trust_events(agent_id);
CREATE INDEX idx_trust_events_type ON trust_events(event_type);
CREATE INDEX idx_trust_events_created ON trust_events(created_at);

COMMENT ON TABLE trust_events IS 'Tracks trust score changes and agentic drift events';

CREATE TABLE hitl_approvals (
    approval_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    resource_id UUID REFERENCES resources(resource_id),
    action VARCHAR(255) NOT NULL,
    params JSONB DEFAULT '{}',
    status VARCHAR(20) DEFAULT 'pending',
    approver_id UUID,
    approval_method VARCHAR(50),
    approved_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_hitl_status ON hitl_approvals(status);
CREATE INDEX idx_hitl_agent ON hitl_approvals(agent_id);
CREATE INDEX idx_hitl_expires ON hitl_approvals(expires_at);

COMMENT ON TABLE hitl_approvals IS 'Human-in-the-loop approval queue for high-risk actions';