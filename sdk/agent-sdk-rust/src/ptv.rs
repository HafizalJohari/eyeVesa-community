use crate::client::AgentClient;
use crate::PtvAttestResult;

#[derive(Debug, thiserror::Error)]
pub enum PtvError {
    #[error("Attestation failed: {0}")]
    AttestFailed(String),
    #[error("Bind failed: {0}")]
    BindFailed(String),
    #[error("Verify failed: {0}")]
    VerifyFailed(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(serde::Deserialize)]
struct AttestResponse {
    agent_id: String,
    platform: String,
    tpm_signature: String,
    quote: String,
    nonce: String,
}

#[derive(serde::Deserialize)]
struct BindResponse {
    binding_id: String,
    agent_id: String,
    platform: String,
    transformed_at: i64,
    expires_at: i64,
}

#[derive(serde::Deserialize)]
struct VerifyResponse {
    valid: bool,
    #[allow(dead_code)]
    binding_id: String,
}

impl AgentClient {
    pub async fn attest(
        &self,
        platform: &str,
        firmware_version: &str,
    ) -> Result<PtvAttestResult, PtvError> {
        tracing::info!("Attesting identity for platform: {}", platform);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/ptv/attest", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
            "platform": platform,
            "firmware_version": firmware_version,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(PtvError::AttestFailed(format!("{}: {}", status, text)));
        }

        let result: AttestResponse = resp.json().await
            .map_err(|e| PtvError::AttestFailed(e.to_string()))?;

        Ok(PtvAttestResult {
            attestation: serde_json::json!({
                "agent_id": result.agent_id,
                "platform": result.platform,
                "nonce": result.nonce,
            }),
            tpm_signature: result.tpm_signature,
            quote: result.quote,
        })
    }

    pub async fn bind(
        &self,
        attestation: &serde_json::Value,
        tpm_signature: &str,
        platform: &str,
        firmware_version: &str,
        agent_id: &str,
    ) -> Result<crate::PtvBindResult, PtvError> {
        tracing::info!("Binding identity for agent: {}", agent_id);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/ptv/bind", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "agent_id": agent_id,
            "platform": platform,
            "firmware_version": firmware_version,
            "tpm_signature": tpm_signature,
            "attestation": attestation,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(PtvError::BindFailed(format!("{}: {}", status, text)));
        }

        let result: BindResponse = resp.json().await
            .map_err(|e| PtvError::BindFailed(e.to_string()))?;

        Ok(crate::PtvBindResult {
            binding_id: uuid::Uuid::parse_str(&result.binding_id).unwrap_or_default(),
            agent_id: result.agent_id,
            platform: result.platform,
            transformed_at: result.transformed_at,
            expires_at: result.expires_at,
        })
    }

    pub async fn verify_binding(&self, binding_id: &uuid::Uuid) -> Result<bool, PtvError> {
        tracing::info!("Verifying binding: {}", binding_id);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/ptv/verify/{}", gateway.trim_end_matches('/'), binding_id);

        let resp = self.http_client()
            .get(&url)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            return Err(PtvError::VerifyFailed(format!("{}: {}", status, text)));
        }

        let result: VerifyResponse = resp.json().await
            .map_err(|e| PtvError::VerifyFailed(e.to_string()))?;

        Ok(result.valid)
    }
}