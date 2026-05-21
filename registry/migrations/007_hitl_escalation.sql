-- eyeVesa: Multi-layer HITL Escalation & Approval Chains
-- Phase 3: Pro Feature (BSL 1.1 Licensed)

-- Extend hitl_approvals with escalation support
ALTER TABLE hitl_approvals ADD COLUMN IF NOT EXISTS risk_level VARCHAR(20) DEFAULT 'medium';
ALTER TABLE hitl_approvals ADD COLUMN IF NOT EXISTS required_approvals INTEGER DEFAULT 1;
ALTER TABLE hitl_approvals ADD COLUMN IF NOT EXISTS current_approvals INTEGER DEFAULT 0;
ALTER TABLE hitl_approvals ADD COLUMN IF NOT EXISTS escalation_level INTEGER DEFAULT 0;

-- Approval chain: tracks each person in a multi-approval flow
CREATE TABLE hitl_approval_chain (
    chain_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    approval_id UUID NOT NULL REFERENCES hitl_approvals(approval_id),
    approver_id UUID NOT NULL,
    approval_level INTEGER NOT NULL DEFAULT 1,
    decision VARCHAR(20) NOT NULL DEFAULT 'pending',
    decision_reason TEXT,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(approval_id, approval_level, approver_id)
);

CREATE INDEX idx_approval_chain_approval ON hitl_approval_chain(approval_id);
CREATE INDEX idx_approval_chain_approver ON hitl_approval_chain(approver_id);
CREATE INDEX idx_approval_chain_pending ON hitl_approval_chain(approval_id) WHERE decision = 'pending';

-- Notification log: tracks every notification sent for escalation
CREATE TABLE hitl_notifications (
    notification_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    approval_id UUID NOT NULL REFERENCES hitl_approvals(approval_id),
    channel VARCHAR(50) NOT NULL,
    recipient_id UUID NOT NULL,
    message TEXT,
    sent_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    escalation_level INTEGER DEFAULT 0
);

CREATE INDEX idx_hitl_notifications_approval ON hitl_notifications(approval_id);
CREATE INDEX idx_hitl_notifications_pending ON hitl_notifications(approval_id) WHERE acknowledged_at IS NULL;

-- Escalation config: per-tenant escalation timing (tenant_id FK added by 008 migration)
CREATE TABLE hitl_escalation_config (
    config_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID,
    level INT NOT NULL DEFAULT 0,
    timeout_seconds INT NOT NULL DEFAULT 300,
    notify_channel VARCHAR(50) NOT NULL DEFAULT 'webhook',
    notify_target TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE hitl_approval_chain IS 'Multi-approver chain for Layer 4 escalation — tracks each approver decision';
COMMENT ON TABLE hitl_notifications IS 'Notification delivery log — tracks Slack/webhook/email notifications for HITL';
COMMENT ON TABLE hitl_escalation_config IS 'Escalation timing and channel configuration per tenant';