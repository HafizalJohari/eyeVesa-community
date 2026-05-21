-- AgentID Gateway: Transaction Token Revocation Store
-- Stores revoked capability tokens for transaction protocol

CREATE TABLE revoked_tokens (
    token_id VARCHAR(64) PRIMARY KEY,
    reason TEXT NOT NULL DEFAULT '',
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_revoked_tokens_revoked_at ON revoked_tokens(revoked_at);

COMMENT ON TABLE revoked_tokens IS 'Revoked capability tokens preventing replay after revocation';