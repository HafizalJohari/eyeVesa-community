use crate::AgentConfig;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use ed25519_dalek::SigningKey;
use reqwest::Client;
use std::sync::RwLock;

pub struct AgentClient {
    config: RwLock<AgentConfig>,
    signing_key: SigningKey,
    trust_score: RwLock<f64>,
    http: Client,
    registered: RwLock<bool>,
    jwt_token: RwLock<Option<String>>,
    api_key: Option<String>,
}

impl AgentClient {
    pub fn new(config: AgentConfig, signing_key: SigningKey) -> Self {
        Self {
            config: RwLock::new(config),
            signing_key,
            trust_score: RwLock::new(1.0),
            registered: RwLock::new(false),
            http: Client::new(),
            jwt_token: RwLock::new(None),
            api_key: None,
        }
    }

    pub fn with_api_key(mut self, api_key: String) -> Self {
        self.api_key = Some(api_key);
        self
    }

    pub fn agent_id(&self) -> uuid::Uuid {
        self.config.read().unwrap().agent_id
    }

    pub fn trust_score(&self) -> f64 {
        *self.trust_score.read().unwrap()
    }

    pub fn update_trust_score(&self, score: f64) {
        *self.trust_score.write().unwrap() = score;
    }

    pub fn gateway_endpoint(&self) -> String {
        self.config.read().unwrap().gateway_endpoint.clone()
    }

    pub fn name(&self) -> String {
        self.config.read().unwrap().name.clone()
    }

    pub fn owner(&self) -> String {
        self.config.read().unwrap().owner.clone()
    }

    pub fn signing_key(&self) -> &SigningKey {
        &self.signing_key
    }

    pub fn is_registered(&self) -> bool {
        *self.registered.read().unwrap()
    }

    pub fn public_key_base64(&self) -> String {
        BASE64.encode(self.signing_key.verifying_key().to_bytes())
    }

    pub fn jwt_token(&self) -> Option<String> {
        self.jwt_token.read().unwrap().clone()
    }

    pub(crate) fn http_client(&self) -> &Client {
        &self.http
    }

    pub(crate) fn set_registered(&self, registered: bool) {
        *self.registered.write().unwrap() = registered;
    }

    pub(crate) fn set_agent_id(&self, id: uuid::Uuid) {
        self.config.write().unwrap().agent_id = id;
    }

    pub(crate) fn set_jwt_token(&self, token: String) {
        *self.jwt_token.write().unwrap() = Some(token);
    }

    pub(crate) fn auth_headers(&self) -> reqwest::header::HeaderMap {
        let mut headers = reqwest::header::HeaderMap::new();
        headers.insert("Content-Type", "application/json".parse().unwrap());
        if let Some(ref token) = *self.jwt_token.read().unwrap() {
            headers.insert("Authorization", format!("Bearer {}", token).parse().unwrap());
        }
        if let Some(ref key) = self.api_key {
            headers.insert("X-API-Key", key.parse().unwrap());
        }
        headers
    }
}