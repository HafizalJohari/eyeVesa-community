-- eyeVesa Federation: Differentiate domestic (local) vs international (remote) peers
-- Migration 020: Add peer_type column to federation_peers

-- peer_type distinguishes:
--   'self'     = this gateway's own registration (the local airport)
--   'domestic' = a peer on the same local network / same deployment
--   'remote'   = a peer at a different Central Airport (international)
ALTER TABLE federation_peers ADD COLUMN IF NOT EXISTS peer_type TEXT NOT NULL DEFAULT 'remote'
    CHECK (peer_type IN ('self', 'domestic', 'remote'));

-- When a gateway registers itself with the Central Airport, it should be
-- marked as 'self' on its own DB and 'remote' on the Central Airport's DB.
-- When an admin registers a known gateway on the same network, mark as 'domestic'.

CREATE INDEX IF NOT EXISTS idx_federation_peers_peer_type ON federation_peers(peer_type);

-- Add endpoint_type to distinguish domestic (local) vs international (remote) routes
-- on federated_agents so queries can filter by scope
ALTER TABLE federated_agents ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'international'
    CHECK (scope IN ('domestic', 'international'));

-- Agents synced from the local gateway's own agents should have scope='domestic'
-- Agents synced from a remote gateway should have scope='international'

CREATE INDEX IF NOT EXISTS idx_federated_agents_scope ON federated_agents(scope);