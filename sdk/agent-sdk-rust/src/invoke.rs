use crate::client::AgentClient;
use crate::{AuthorizeResult, InvokeResult};
use ed25519_dalek::Signer;

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
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

impl AgentClient {
    pub async fn invoke(
        &self,
        resource_id: &uuid::Uuid,
        tool: &str,
        _params: serde_json::Value,
    ) -> Result<InvokeResult, InvokeError> {
        tracing::info!("Invoking tool {} on resource {}", tool, resource_id);

        let gateway = self.gateway_endpoint();
        let auth_url = format!("{}/v1/auth", gateway.trim_end_matches('/'));

        let auth_body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
            "action": tool,
            "resource_id": resource_id.to_string(),
        });

        let auth_resp = self.http_client()
            .post(&auth_url)
            .json(&auth_body)
            .send()
            .await?;

        if !auth_resp.status().is_success() {
            return Err(InvokeError::Gateway(format!("auth request failed: {}", auth_resp.status())));
        }

        let auth_result: AuthorizeResult = auth_resp.json().await
            .map_err(|e| InvokeError::Gateway(e.to_string()))?;

        if !auth_result.allowed {
            if auth_result.requires_hitl {
                return Err(InvokeError::HitlRequired(auth_result.reason));
            }
            return Err(InvokeError::NotAuthorized(auth_result.reason));
        }

        let mcp_url = format!("{}/v1/mcp", gateway.trim_end_matches('/'));

        let mcp_body = serde_json::json!({
            "jsonrpc": "2.0",
            "method": "tools/call",
            "id": 1,
            "params": {
                "name": tool,
                "arguments": {
                    "agent_id": self.agent_id().to_string(),
                    "resource_id": resource_id.to_string(),
                }
            }
        });

        let payload_bytes = serde_json::to_vec(&mcp_body).unwrap();
        let _signature = self.signing_key().sign(&payload_bytes);

        let mcp_resp = self.http_client()
            .post(&mcp_url)
            .json(&mcp_body)
            .send()
            .await?;

        if !mcp_resp.status().is_success() {
            return Err(InvokeError::Gateway(format!("MCP request failed: {}", mcp_resp.status())));
        }

        let mcp_result: serde_json::Value = mcp_resp.json().await
            .map_err(|e| InvokeError::Gateway(e.to_string()))?;

        let result_data = mcp_result.get("result")
            .cloned()
            .unwrap_or(serde_json::json!({"status": "invoked"}));

        Ok(InvokeResult {
            success: true,
            data: result_data,
            trust_score: self.trust_score(),
        })
    }
}