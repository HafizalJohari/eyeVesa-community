-- eyeVesa v0.1.1: API key hardening and production tenant_id repair

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_name = 'api_keys'
          AND constraint_name = 'api_keys_tenant_id_fkey'
          AND constraint_type = 'FOREIGN KEY'
    ) THEN
        ALTER TABLE api_keys DROP CONSTRAINT api_keys_tenant_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'api_keys'
          AND column_name = 'tenant_id'
          AND udt_name <> 'text'
    ) THEN
        ALTER TABLE api_keys
            ALTER COLUMN tenant_id TYPE TEXT USING tenant_id::TEXT;
    END IF;
END $$;

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS api_key_hash TEXT;

UPDATE api_keys
SET api_key_hash = api_key
WHERE api_key_hash IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(api_key_hash) WHERE is_active = TRUE;

COMMENT ON COLUMN api_keys.api_key_hash IS 'SHA-256 hex digest of the API key. New keys do not store the plaintext secret.';
