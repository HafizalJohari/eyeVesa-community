-- AgentID Gateway: Skills Registry
-- Stores skill definitions, agent-skill assignments, per-skill trust scores, and endorsements

CREATE TABLE skills (
    skill_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(50) NOT NULL DEFAULT 'general',
    risk_level VARCHAR(20) NOT NULL DEFAULT 'medium',
    required_trust_min DECIMAL(5,4) NOT NULL DEFAULT 0.5000,
    required_proficiency SMALLINT NOT NULL DEFAULT 1 CHECK (required_proficiency BETWEEN 1 AND 5),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_skills_category ON skills(category);
CREATE INDEX idx_skills_risk_level ON skills(risk_level);

CREATE TABLE agent_skills (
    agent_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(skill_id) ON DELETE CASCADE,
    proficiency SMALLINT NOT NULL DEFAULT 1 CHECK (proficiency BETWEEN 1 AND 5),
    verified BOOLEAN NOT NULL DEFAULT false,
    verified_by VARCHAR(255),
    verified_at TIMESTAMPTZ,
    endorsements_count INT NOT NULL DEFAULT 0,
    acquired_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);

CREATE INDEX idx_agent_skills_agent ON agent_skills(agent_id);
CREATE INDEX idx_agent_skills_skill ON agent_skills(skill_id);
CREATE INDEX idx_agent_skills_verified ON agent_skills(verified);

CREATE TABLE skill_trust_scores (
    agent_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(skill_id) ON DELETE CASCADE,
    trust_score DECIMAL(5,4) NOT NULL DEFAULT 1.0000 CHECK (trust_score BETWEEN 0 AND 1),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (agent_id, skill_id)
);

CREATE INDEX idx_skill_trust_agent ON skill_trust_scores(agent_id);

CREATE TABLE skill_endorsements (
    endorsement_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    skill_id UUID NOT NULL REFERENCES skills(skill_id) ON DELETE CASCADE,
    endorser_type VARCHAR(20) NOT NULL CHECK (endorser_type IN ('human', 'agent', 'ptv')),
    endorser_id VARCHAR(255) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_skill_endorsements_agent ON skill_endorsements(agent_id);
CREATE INDEX idx_skill_endorsements_skill ON skill_endorsements(skill_id);

-- Auto-verify skill when endorsements reach threshold (3)
CREATE OR REPLACE FUNCTION auto_verify_skill() RETURNS TRIGGER AS $$
BEGIN
    UPDATE agent_skills
    SET verified = true, verified_by = 'auto:endorsements>=3', verified_at = NOW()
    WHERE agent_id = NEW.agent_id AND skill_id = NEW.skill_id
      AND endorsements_count >= 2;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_auto_verify_skill
    AFTER INSERT ON skill_endorsements
    FOR EACH ROW
    EXECUTE FUNCTION auto_verify_skill();

-- Increment endorsements_count on new endorsement
CREATE OR REPLACE FUNCTION increment_endorsements() RETURNS TRIGGER AS $$
BEGIN
    UPDATE agent_skills
    SET endorsements_count = endorsements_count + 1
    WHERE agent_id = NEW.agent_id AND skill_id = NEW.skill_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_increment_endorsements
    AFTER INSERT ON skill_endorsements
    FOR EACH ROW
    EXECUTE FUNCTION increment_endorsements();

COMMENT ON TABLE skills IS 'Master skill catalog defining available competencies';
COMMENT ON TABLE agent_skills IS 'Agent-skill assignments with proficiency and verification status';
COMMENT ON TABLE skill_trust_scores IS 'Per-skill trust scores overriding global agent trust';
COMMENT ON TABLE skill_endorsements IS 'Audit trail of skill endorsements from humans, agents, or PTV attestations';

-- Add required_skills column to resources
ALTER TABLE resources ADD COLUMN IF NOT EXISTS required_skills TEXT[] DEFAULT '{}';