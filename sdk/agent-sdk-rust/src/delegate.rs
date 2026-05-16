use crate::client::AgentClient;
use crate::DelegateResult;

#[derive(Debug, thiserror::Error)]
pub enum DelegateError {
    #[error("Delegation not allowed: {0}")]
    NotAllowed(String),
    #[error("Max depth exceeded: {0}")]
    MaxDepth(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(serde::Deserialize)]
struct DelegateResponse {
    delegation_id: String,
    status: String,
}

impl AgentClient {
    pub async fn delegate(
        &self,
        delegatee_id: &uuid::Uuid,
        scope: Vec<String>,
        reason: &str,
    ) -> Result<DelegateResult, DelegateError> {
        tracing::info!(
            "Delegating to agent {} with scope {:?}",
            delegatee_id, scope
        );

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/delegate", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "delegator_id": self.agent_id().to_string(),
            "delegatee_id": delegatee_id.to_string(),
            "scope": scope,
            "reason": reason,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            if text.contains("max depth") || text.contains("depth") {
                return Err(DelegateError::MaxDepth(text));
            }
            return Err(DelegateError::NotAllowed(format!("{}: {}", status, text)));
        }

        let result: DelegateResponse = resp.json().await
            .map_err(|e| DelegateError::Gateway(e.to_string()))?;

        Ok(DelegateResult {
            delegation_id: uuid::Uuid::parse_str(&result.delegation_id).unwrap_or_default(),
            status: result.status,
        })
    }
}