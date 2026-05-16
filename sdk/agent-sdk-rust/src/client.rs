use crate::{AgentConfig, ToolInfo, InvokeResult};
use ed25519_dalek::SigningKey;

pub struct AgentClient {
    config: AgentConfig,
    signing_key: SigningKey,
    trust_score: f64,
}

impl AgentClient {
    pub fn new(config: AgentConfig, signing_key: SigningKey) -> Self {
        Self {
            config,
            signing_key,
            trust_score: 1.0,
        }
    }

    pub fn agent_id(&self) -> &uuid::Uuid {
        &self.config.agent_id
    }

    pub fn trust_score(&self) -> f64 {
        self.trust_score
    }

    pub fn update_trust_score(&mut self, score: f64) {
        self.trust_score = score;
    }
}