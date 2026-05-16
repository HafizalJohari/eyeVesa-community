use crate::client::AgentClient;
use crate::AgentConfig;

#[derive(Debug, thiserror::Error)]
pub enum ConnectError {
    #[error("TLS error: {0}")]
    Tls(String),
    #[error("Connection refused: {0}")]
    ConnectionRefused(String),
    #[error("Authentication failed: {0}")]
    AuthFailed(String),
}

impl AgentClient {
    pub async fn connect(
        config: AgentConfig,
        signing_key: ed25519_dalek::SigningKey,
    ) -> Result<Self, ConnectError> {
        tracing::info!("Connecting to gateway at {}", config.gateway_endpoint);

        let client = Self::new(config, signing_key);

        tracing::info!("Agent {} connected", client.agent_id());

        Ok(client)
    }
}