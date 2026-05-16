-- eyeVesa: Budget Metering & Rate Limiting Tables
-- Phase 3: Pro Feature (BSL 1.1 Licensed)

-- Spend tracking: cumulative budget enforcement
CREATE TABLE agent_spend (
    spend_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    resource_id UUID REFERENCES resources(resource_id),
    action VARCHAR(255) NOT NULL,
    estimated_cost DECIMAL(10,4) NOT NULL DEFAULT 0,
    actual_cost DECIMAL(10,4) DEFAULT 0,
    period VARCHAR(20) NOT NULL DEFAULT 'monthly',
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_spend_agent_period ON agent_spend(agent_id, period_start, period_end);
CREATE INDEX idx_agent_spend_resource ON agent_spend(resource_id);

-- Rate limit counters: per-agent rate limiting
CREATE TABLE rate_limit_counters (
    counter_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    resource_id UUID REFERENCES resources(resource_id),
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    request_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_rate_limit_agent_window ON rate_limit_counters(agent_id, window_start, window_end);

COMMENT ON TABLE agent_spend IS 'Cumulative spend tracking for budget enforcement per agent';
COMMENT ON TABLE rate_limit_counters IS 'Sliding window rate limit counters per agent-resource pair';