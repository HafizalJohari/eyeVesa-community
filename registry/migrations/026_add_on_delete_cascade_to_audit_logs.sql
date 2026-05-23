-- Add ON DELETE CASCADE to audit_logs.agent_id foreign key

ALTER TABLE audit_logs
DROP CONSTRAINT audit_logs_agent_id_fkey,
ADD CONSTRAINT audit_logs_agent_id_fkey
FOREIGN KEY (agent_id)
REFERENCES agents(agent_id)
ON DELETE CASCADE;
