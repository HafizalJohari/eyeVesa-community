-- eyeVesa: Push Notification Device Tokens
-- Phase 3: HITL Mobile Push (APNs/FCM)

CREATE TABLE push_tokens (
    token_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    approver_id UUID NOT NULL REFERENCES approvers(approver_id),
    device_token VARCHAR(500) NOT NULL,
    platform VARCHAR(20) NOT NULL DEFAULT 'ios',
    bundle_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,
    UNIQUE(approver_id, device_token)
);

CREATE INDEX idx_push_tokens_approver ON push_tokens(approver_id);
CREATE INDEX idx_push_tokens_active ON push_tokens(approver_id) WHERE is_active = TRUE;

COMMENT ON TABLE push_tokens IS 'Device push tokens for HITL approval notifications (APNs/FCM)';