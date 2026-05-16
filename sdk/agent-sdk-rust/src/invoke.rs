use crate::client::AgentClient;
use crate::InvokeResult;

#[derive(Debug, thiserror::Error)]
pub enum InvokeError {
    #[error("Not authorized: {0}")]
    NotAuthorized(String),
    #[error("Resource unavailable: {0}")]
    ResourceUnavailable(String),
    #[error("Human approval required: {0}")]
    HitlRequired(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
}

impl AgentClient {
    pub async fn invoke(
        &self,
        resource_id: &uuid::Uuid,
        tool: &str,
        params: serde_json::Value,
    ) -> Result<InvokeResult, InvokeError> {
        tracing::info!("Invoking tool {} on resource {}", tool, resource_id);

        let payload = serde_json::json!({
            "jsonrpc": "2.0",
            "method": "tools/call",
            "id": 1,
            "params": {
                "name": tool,
                "arguments": params
            }
        });

        let payload_bytes = serde_json::to_vec(&payload).unwrap();
        let _signature = self.signing_key.sign(&payload_bytes);

        Ok(InvokeResult {
            success: true,
            data: serde_json::json!({"status": "stub"}),
            trust_score: self.trust_score,
        })
    }
}