-- eyeVesa Airport: Where agents meet
-- Migration 017: Agent profiles, heartbeats, and airport search

-- Agent heartbeat (online/offline tracking)
CREATE TABLE IF NOT EXISTS agent_heartbeats (
    agent_id UUID PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    last_heartbeat TIMESTAMP NOT NULL DEFAULT NOW(),
    status TEXT NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline', 'busy', 'idle')),
    metadata JSONB DEFAULT '{}',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_heartbeats_status ON agent_heartbeats(status);
CREATE INDEX idx_agent_heartbeats_last ON agent_heartbeats(last_heartbeat DESC);

-- Agent profiles (extended info for airport directory)
CREATE TABLE IF NOT EXISTS agent_profiles (
    agent_id UUID PRIMARY KEY REFERENCES agents(agent_id) ON DELETE CASCADE,
    description TEXT DEFAULT '',
    services_offered JSONB DEFAULT '[]',
    endpoints JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    total_actions INT DEFAULT 0,
    approval_rate FLOAT DEFAULT 1.0,
    avg_response_ms INT DEFAULT 0,
    listed BOOLEAN DEFAULT true,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_profiles_listed ON agent_profiles(listed) WHERE listed = true;
CREATE INDEX idx_agent_profiles_tags ON agent_profiles USING GIN(tags);

-- Airport connection log (who met whom)
CREATE TABLE IF NOT EXISTS airport_connections (
    connection_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    responder_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    outcome TEXT NOT NULL DEFAULT 'success' CHECK (outcome IN ('success', 'denied', 'hitl_required', 'timeout', 'error')),
    trust_score_at_time FLOAT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_airport_requester ON airport_connections(requester_id);
CREATE INDEX idx_airport_responder ON airport_connections(responder_id);
CREATE INDEX idx_airport_created ON airport_connections(created_at DESC);

-- Function: mark agents offline if heartbeat older than 2 minutes
CREATE OR REPLACE FUNCTION airport_mark_stale_offline()
RETURNS void AS $$
BEGIN
    UPDATE agent_heartbeats
    SET status = 'offline', updated_at = NOW()
    WHERE last_heartbeat < NOW() - INTERVAL '2 minutes'
      AND status != 'offline';
END;
$$ LANGUAGE plpgsql;