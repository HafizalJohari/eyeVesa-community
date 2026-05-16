use serde::Deserialize;

#[derive(Debug, Clone, Deserialize)]
pub struct GatewayIdentity {
    pub spiffe_id: String,
    pub trust_domain: String,
    pub expires_at: String,
}

pub async fn fetch_identity(control_plane_http: &str) -> Result<GatewayIdentity, Box<dyn std::error::Error + Send + Sync>> {
    let url = format!("http://{}/identity", control_plane_http);
    let client = reqwest::Client::new();
    let resp = client.get(&url).send().await?;

    if !resp.status().is_success() {
        return Err(format!("identity endpoint returned {}", resp.status()).into());
    }

    let identity: GatewayIdentity = resp.json().await?;
    Ok(identity)
}