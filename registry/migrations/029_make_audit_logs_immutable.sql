-- eyeVesa: Make audit logs immutable
-- Enforce write-once semantics for audit trail integrity

-- Create a function to prevent deletions
CREATE OR REPLACE FUNCTION prevent_audit_log_deletion() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is immutable: deletion of audit records is not permitted. All audit entries must be retained for compliance.';
END;
$$ LANGUAGE plpgsql;

-- Create trigger to prevent DELETE operations on audit_logs
DROP TRIGGER IF EXISTS audit_logs_immutable_delete ON audit_logs;
CREATE TRIGGER audit_logs_immutable_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_log_deletion();

-- Create a function to prevent updates
CREATE OR REPLACE FUNCTION prevent_audit_log_update() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is immutable: audit records cannot be modified. Log entries are write-once.';
END;
$$ LANGUAGE plpgsql;

-- Create trigger to prevent UPDATE operations on audit_logs
DROP TRIGGER IF EXISTS audit_logs_immutable_update ON audit_logs;
CREATE TRIGGER audit_logs_immutable_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_log_update();

-- Add comment documenting immutability
COMMENT ON TABLE audit_logs IS 'Write-once immutable audit trail. Records cannot be updated or deleted after creation. Cryptographically signed for integrity verification.';
COMMENT ON FUNCTION prevent_audit_log_deletion() IS 'Prevents deletion of audit log records to ensure audit trail integrity';
COMMENT ON FUNCTION prevent_audit_log_update() IS 'Prevents modification of audit log records to ensure audit trail integrity';
