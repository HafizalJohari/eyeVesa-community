-- Add unique constraint on federation_peers.endpoint for ON CONFLICT upsert
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'federation_peers_endpoint_key'
    ) THEN
        ALTER TABLE federation_peers ADD CONSTRAINT federation_peers_endpoint_key UNIQUE (endpoint);
    END IF;
END $$;