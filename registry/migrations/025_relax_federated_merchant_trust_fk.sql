-- Allow merchant trust sync before federated agent passport sync.
-- Keep gateway FK and dedupe protections; remove strict merchant->federated_agents FK.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_name = 'federated_merchant_trust'
          AND constraint_name = 'federated_merchant_trust_merchant_id_fkey'
          AND constraint_type = 'FOREIGN KEY'
    ) THEN
        ALTER TABLE federated_merchant_trust
            DROP CONSTRAINT federated_merchant_trust_merchant_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_name = 'federated_merchant_trust_events'
          AND constraint_name = 'federated_merchant_trust_events_merchant_id_fkey'
          AND constraint_type = 'FOREIGN KEY'
    ) THEN
        ALTER TABLE federated_merchant_trust_events
            DROP CONSTRAINT federated_merchant_trust_events_merchant_id_fkey;
    END IF;
END $$;
