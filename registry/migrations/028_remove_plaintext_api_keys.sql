-- eyeVesa: Remove plaintext API keys
-- Deprecate plaintext api_key storage for better security
-- Only use api_key_hash for lookups going forward

-- Clear plaintext keys that match their hashes (safe to remove)
UPDATE api_keys
SET api_key = NULL
WHERE api_key_hash IS NOT NULL;

-- Drop the unique constraint on plaintext key
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_api_key_key;

-- Drop the index on plaintext key
DROP INDEX IF EXISTS idx_api_keys_key;

-- Allow new hashed-only keys to omit deprecated plaintext storage.
ALTER TABLE api_keys
    ALTER COLUMN api_key DROP NOT NULL;

-- Make api_key_hash NOT NULL (all keys must have hash now)
ALTER TABLE api_keys
    ALTER COLUMN api_key_hash SET NOT NULL;

-- Add a unique partial index on active api_key_hash values.
CREATE UNIQUE INDEX IF NOT EXISTS uk_api_keys_hash_active
    ON api_keys (api_key_hash)
    WHERE is_active = TRUE;

COMMENT ON COLUMN api_keys.api_key IS 'DEPRECATED: No longer used. Use api_key_hash for authentication. Kept for backward compatibility during migration period.';
