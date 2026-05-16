use crate::client::AgentClient;
use crate::HitlApproval;

#[derive(Debug, thiserror::Error)]
pub enum HitlError {
    #[error("Approval request failed: {0}")]
    RequestFailed(String),
    #[error("Approval not found: {0}")]
    NotFound(String),
    #[error("Decision failed: {0}")]
    DecisionFailed(String),
    #[error("Pending query failed: {0}")]
    PendingFailed(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(serde::Deserialize)]
struct ApprovalResponse {
    approval_id: String,
    status: String,
    #[allow(dead_code)]
    reason: Option<String>,
}

impl AgentClient {
    pub async fn request_approval(
        &self,
        action: &str,
        reason: &str,
        risk_level: &str,
    ) -> Result<HitlApproval, HitlError> {
        tracing::info!("Requesting HITL approval for action: {}", action);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/hitl/request", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
            "action": action,
            "reason": reason,
            "risk_level": risk_level,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(HitlError::RequestFailed(format!("{}: {}", status, text)));
        }

        let result: ApprovalResponse = resp.json().await
            .map_err(|e| HitlError::RequestFailed(e.to_string()))?;

        Ok(HitlApproval {
            approval_id: uuid::Uuid::parse_str(&result.approval_id).unwrap_or_default(),
            agent_id: self.agent_id().to_string(),
            action: action.to_string(),
            status: result.status,
            expires_at: None,
        })
    }

    pub async fn decide_approval(
        &self,
        approval_id: &uuid::Uuid,
        approved: bool,
        approver_method: &str,
    ) -> Result<String, HitlError> {
        tracing::info!("Deciding approval {}: approved={}", approval_id, approved);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/hitl/{}/decide", gateway.trim_end_matches('/'), approval_id);

        let body = serde_json::json!({
            "approval_id": approval_id.to_string(),
            "approved": approved,
            "approver_method": approver_method,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(HitlError::DecisionFailed(format!("{}: {}", status, text)));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| HitlError::DecisionFailed(e.to_string()))?;

        Ok(result.get("status").and_then(|s| s.as_str()).unwrap_or("unknown").to_string())
    }

    pub async fn get_approval_status(&self, approval_id: &uuid::Uuid) -> Result<String, HitlError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/hitl/{}", gateway.trim_end_matches('/'), approval_id);

        let resp = self.http_client()
            .get(&url)
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(HitlError::NotFound(approval_id.to_string()));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| HitlError::NotFound(e.to_string()))?;

        Ok(result.get("status").and_then(|s| s.as_str()).unwrap_or("unknown").to_string())
    }

    pub async fn list_pending_approvals(&self) -> Result<Vec<serde_json::Value>, HitlError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/hitl/pending?agent_id={}", gateway.trim_end_matches('/'), self.agent_id());

        let resp = self.http_client()
            .get(&url)
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(HitlError::PendingFailed(format!("status: {}", resp.status())));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| HitlError::PendingFailed(e.to_string()))?;

        Ok(result.get("approvals")
            .and_then(|a| a.as_array())
            .cloned()
            .unwrap_or_default())
    }
}