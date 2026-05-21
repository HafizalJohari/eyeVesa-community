-- Up
CREATE TABLE IF NOT EXISTS system_metadata (
    key VARCHAR(255) PRIMARY KEY,
    value VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Initialize the system high-water mark timestamp.
INSERT INTO system_metadata (key, value)
VALUES ('last_active_time', EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)::TEXT)
ON CONFLICT (key) DO NOTHING;
