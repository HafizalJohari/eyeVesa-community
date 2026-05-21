use crate::client::AgentClient;
use crate::AgentConfig;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use ed25519_dalek::Signer;

#[derive(Debug, thiserror::Error)]
pub enum ConnectError {
    #[error("Connection failed: {0}")]
    ConnectionFailed(String),
    #[error("Authentication failed: {0}")]
    AuthFailed(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(serde::Deserialize)]
struct RegisterResponse {
    agent_id: String,
    #[allow(dead_code)]
    public_key: String,
    status: String,
    trust_score: f64,
}

#[derive(serde::Deserialize)]
struct ChallengeResponse {
    nonce: String,
}

#[derive(serde::Deserialize)]
struct LoginResponse {
    token: String,
    #[allow(dead_code)]
    expires_at: Option<String>,
}

#[derive(serde::Deserialize)]
struct CreateApiKeyResponse {
    api_key: String,
    #[allow(dead_code)]
    name: String,
    #[allow(dead_code)]
    id: String,
}

impl AgentClient {
    pub async fn register(&self) -> Result<(), ConnectError> {
        tracing::info!("Registering agent {} at {}", self.name(), self.gateway_endpoint());

        let url = format!("{}/v1/agents/register", self.gateway_endpoint().trim_end_matches('/'));
        let body = serde_json::json!({
            "name": self.name(),
            "owner": self.owner(),
            "public_key": self.public_key_base64(),
            "capabilities": ["mcp"],
            "allowed_tools": ["read", "get_weather", "search_docs"],
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await
            .map_err(|e| ConnectError::ConnectionFailed(e.to_string()))?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(ConnectError::AuthFailed(format!("{}: {}", status, text)));
        }

        let reg: RegisterResponse = resp.json().await
            .map_err(|e| ConnectError::AuthFailed(e.to_string()))?;

        self.update_trust_score(reg.trust_score);
        self.set_registered(true);

        if let Ok(id) = uuid::Uuid::parse_str(&reg.agent_id) {
            self.set_agent_id(id);
        }

        tracing::info!("Agent {} registered (id: {})", self.name(), reg.agent_id);
        Ok(())
    }

    pub async fn login(&self) -> Result<(), ConnectError> {
        tracing::info!("Authenticating agent {} via challenge-response", self.agent_id());

        let url = format!("{}/v1/auth/challenge", self.gateway_endpoint().trim_end_matches('/'));
        let body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await
            .map_err(|e| ConnectError::ConnectionFailed(e.to_string()))?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(ConnectError::AuthFailed(format!("challenge failed: {}: {}", status, text)));
        }

        let challenge: ChallengeResponse = resp.json().await
            .map_err(|e| ConnectError::AuthFailed(e.to_string()))?;

        let nonce = challenge.nonce;
        let signature = self.signing_key().sign(nonce.as_bytes());
        let signature_b64 = BASE64.encode(signature.to_bytes());

        let login_url = format!("{}/v1/auth/login", self.gateway_endpoint().trim_end_matches('/'));
        let login_body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
            "nonce": nonce,
            "signature": signature_b64,
        });

        let login_resp = self.http_client()
            .post(&login_url)
            .json(&login_body)
            .send()
            .await
            .map_err(|e| ConnectError::ConnectionFailed(e.to_string()))?;

        if !login_resp.status().is_success() {
            let status = login_resp.status();
            let text = login_resp.text().await.unwrap_or_default();
            return Err(ConnectError::AuthFailed(format!("login failed: {}: {}", status, text)));
        }

        let login_data: LoginResponse = login_resp.json().await
            .map_err(|e| ConnectError::AuthFailed(e.to_string()))?;

        self.set_jwt_token(login_data.token);

        tracing::info!("Agent {} authenticated", self.agent_id());
        Ok(())
    }

    pub async fn connect(
        config: AgentConfig,
        signing_key: ed25519_dalek::SigningKey,
    ) -> Result<Self, ConnectError> {
        let client = Self::new(config, signing_key);
        client.register().await?;
        client.login().await?;
        Ok(client)
    }

    pub async fn create_api_key(&self, name: &str, tenant_id: Option<&str>) -> Result<String, ConnectError> {
        let url = format!("{}/v1/api-keys", self.gateway_endpoint().trim_end_matches('/'));
        let mut body = serde_json::json!({ "name": name });
        if let Some(tid) = tenant_id {
            body["tenant_id"] = serde_json::Value::String(tid.to_string());
        }

        let resp = self.http_client()
            .post(&url)
            .headers(self.auth_headers())
            .json(&body)
            .send()
            .await
            .map_err(|e| ConnectError::ConnectionFailed(e.to_string()))?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(ConnectError::AuthFailed(format!("create API key failed: {}: {}", status, text)));
        }

        let data: CreateApiKeyResponse = resp.json().await
            .map_err(|e| ConnectError::AuthFailed(e.to_string()))?;

        Ok(data.api_key)
    }
}