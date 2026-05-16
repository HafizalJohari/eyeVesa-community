use crate::AgentConfig;
use ed25519_dalek::SigningKey;
use reqwest::Client;

pub struct AgentClient {
    config: AgentConfig,
    signing_key: SigningKey,
    trust_score: f64,
    http: Client,
    registered: bool,
}

impl AgentClient {
    pub fn new(config: AgentConfig, signing_key: SigningKey) -> Self {
        Self {
            config,
            signing_key,
            trust_score: 1.0,
            registered: false,
            http: Client::new(),
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

    pub fn gateway_endpoint(&self) -> &str {
        &self.config.gateway_endpoint
    }

    pub fn name(&self) -> &str {
        &self.config.name
    }

    pub fn owner(&self) -> &str {
        &self.config.owner
    }

    pub fn signing_key(&self) -> &SigningKey {
        &self.signing_key
    }

    pub fn is_registered(&self) -> bool {
        self.registered
    }

    pub(crate) fn http_client(&self) -> &Client {
        &self.http
    }

    pub(crate) fn set_registered(&mut self, registered: bool) {
        self.registered = registered;
    }

    pub(crate) fn set_agent_id(&mut self, id: uuid::Uuid) {
        self.config.agent_id = id;
    }
}