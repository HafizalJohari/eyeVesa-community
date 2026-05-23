-- Add ON DELETE CASCADE to trust_events.agent_id foreign key

ALTER TABLE trust_events
DROP CONSTRAINT trust_events_agent_id_fkey,
ADD CONSTRAINT trust_events_agent_id_fkey
FOREIGN KEY (agent_id)
REFERENCES agents(agent_id)
ON DELETE CASCADE;
