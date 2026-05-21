-- eyeVesa: pgvector Behavioral Embeddings
-- Phase 3: Pro Feature (BSL 1.1 Licensed)

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE agents ADD COLUMN IF NOT EXISTS behavior_vec vector(1536);

CREATE INDEX IF NOT EXISTS idx_agents_behavior_vec ON agents USING ivfflat (behavior_vec vector_cosine_ops) WITH (lists = 100);

-- Behavioral events: training data for anomaly detection
CREATE TABLE behavioral_events (
    event_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    tool VARCHAR(255) NOT NULL,
    resource_id UUID REFERENCES resources(resource_id),
    action_outcome VARCHAR(20) NOT NULL,
    params_hash VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_behavioral_events_agent ON behavioral_events(agent_id);
CREATE INDEX idx_behavioral_events_tool ON behavioral_events(tool);
CREATE INDEX idx_behavioral_events_created ON behavioral_events(created_at);

-- Anomaly detections: flagged unusual behavior
CREATE TABLE behavioral_anomalies (
    anomaly_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    similarity_score DECIMAL(5,4) NOT NULL,
    baseline_behavior TEXT NOT NULL,
    detected_behavior TEXT NOT NULL,
    anomaly_type VARCHAR(50) NOT NULL DEFAULT 'drift',
    resolved BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_behavioral_anomalies_agent ON behavioral_anomalies(agent_id);
CREATE INDEX idx_behavioral_anomalies_unresolved ON behavioral_anomalies(agent_id) WHERE resolved = FALSE;

COMMENT ON TABLE behavioral_events IS 'Raw action events used to generate behavioral embedding vectors';
COMMENT ON TABLE behavioral_anomalies IS 'Detected behavioral anomalies via pgvector similarity scoring';