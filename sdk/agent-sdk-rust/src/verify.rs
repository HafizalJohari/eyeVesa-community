use crate::client::AgentClient;

#[derive(Debug, thiserror::Error)]
pub enum VerifyError {
    #[error("Agent not found: {0}")]
    NotFound(String),
    #[error("Invalid signature: {0}")]
    InvalidSignature(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

impl AgentClient {
    pub async fn verify_signature(
        &self,
        agent_id: &uuid::Uuid,
        message: &[u8],
        signature: &[u8],
    ) -> Result<bool, VerifyError> {
        use base64::Engine;
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/verify-signature", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "agent_id": agent_id.to_string(),
            "message": base64::engine::general_purpose::STANDARD.encode(message),
            "signature": base64::engine::general_purpose::STANDARD.encode(signature),
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if resp.status() == reqwest::StatusCode::NOT_FOUND {
            return Err(VerifyError::NotFound(agent_id.to_string()));
        }

        if !resp.status().is_success() {
            return Err(VerifyError::InvalidSignature(format!("status: {}", resp.status())));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| VerifyError::InvalidSignature(e.to_string()))?;

        Ok(result.get("valid").and_then(|v| v.as_bool()).unwrap_or(false))
    }
}