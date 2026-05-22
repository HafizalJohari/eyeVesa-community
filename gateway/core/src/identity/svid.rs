use serde::Deserialize;

use crate::proxy::forward::control_plane_http_base;

#[derive(Debug, Clone, Deserialize)]
pub struct GatewayIdentity {
    pub spiffe_id: String,
    pub trust_domain: String,
    pub expires_at: String,
}

pub async fn fetch_identity(
    control_plane_http: &str,
    backend_tls_enabled: bool,
) -> Result<GatewayIdentity, Box<dyn std::error::Error + Send + Sync>> {
    let url = format!(
        "{}/identity",
        control_plane_http_base(control_plane_http, backend_tls_enabled)
    );
    let client = reqwest::Client::new();
    let resp = client.get(&url).send().await?;

    if !resp.status().is_success() {
        return Err(format!("identity endpoint returned {}", resp.status()).into());
    }

    let identity: GatewayIdentity = resp.json().await?;
    Ok(identity)
}
