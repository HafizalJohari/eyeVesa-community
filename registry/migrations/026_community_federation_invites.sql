-- eyeVesa Community Federation: invite-only secure agent nodes
-- Migration 026: Peer invites for community node registration

CREATE TABLE IF NOT EXISTS federation_peer_invites (
    invite_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    trust_domain TEXT DEFAULT '',
    peer_type TEXT NOT NULL DEFAULT 'community'
        CHECK (peer_type IN ('self', 'domestic', 'remote', 'community')),
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_federation_peer_invites_endpoint
ON federation_peer_invites(endpoint);

CREATE INDEX IF NOT EXISTS idx_federation_peer_invites_active
ON federation_peer_invites(expires_at)
WHERE used_at IS NULL;

ALTER TABLE federation_peers
DROP CONSTRAINT IF EXISTS federation_peers_peer_type_check;

ALTER TABLE federation_peers
ADD CONSTRAINT federation_peers_peer_type_check
CHECK (peer_type IN ('self', 'domestic', 'remote', 'community'));
