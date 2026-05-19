-- Add unique constraint on federation_peers.endpoint for ON CONFLICT upsert
ALTER TABLE federation_peers ADD CONSTRAINT federation_peers_endpoint_key UNIQUE (endpoint);