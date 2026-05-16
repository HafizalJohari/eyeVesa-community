use crate::client::AgentClient;
use crate::AgentConfig;

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

impl AgentClient {
    pub async fn connect(
        config: AgentConfig,
        signing_key: ed25519_dalek::SigningKey,
    ) -> Result<Self, ConnectError> {
        tracing::info!("Connecting to gateway at {}", config.gateway_endpoint);

        let mut client = Self::new(config, signing_key);

        let gateway = client.gateway_endpoint().to_string();
        let url = format!("{}/v1/register", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "name": client.name(),
            "owner": client.owner(),
            "capabilities": ["mcp"],
            "allowed_tools": ["read", "get_weather", "search_docs"],
        });

        let resp = client.http_client()
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

        client.update_trust_score(reg.trust_score);
        client.set_registered(true);

        if let Ok(id) = uuid::Uuid::parse_str(&reg.agent_id) {
            client.set_agent_id(id);
        }

        tracing::info!("Agent {} connected (status: {})", reg.agent_id, reg.status);

        Ok(client)
    }
}