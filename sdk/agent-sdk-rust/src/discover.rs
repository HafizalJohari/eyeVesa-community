use crate::client::AgentClient;
use crate::ToolInfo;

#[derive(Debug, thiserror::Error)]
pub enum DiscoverError {
    #[error("No resources found matching: {0}")]
    NotFound(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(serde::Deserialize)]
#[allow(dead_code)]
struct ResourcesResponse {
    resources: Vec<serde_json::Value>,
}

impl AgentClient {
    pub async fn discover(&self, capability: &str) -> Result<Vec<ToolInfo>, DiscoverError> {
        tracing::info!("Discovering tools for capability: {}", capability);

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/resources", gateway.trim_end_matches('/'));

        let resp = self.http_client()
            .get(&url)
            .query(&[("capability", capability)])
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(DiscoverError::Gateway(format!("discovery failed: {}", resp.status())));
        }

        let body: serde_json::Value = resp.json().await
            .map_err(|e| DiscoverError::Gateway(e.to_string()))?;

        let resources = body.get("resources")
            .and_then(|r| r.as_array())
            .cloned()
            .unwrap_or_default();

        if resources.is_empty() {
            return Err(DiscoverError::NotFound(capability.to_string()));
        }

        let tools: Vec<ToolInfo> = resources.iter().filter_map(|r| {
            let name = r.get("name")?.as_str()?.to_string();
            let resource_id = r.get("resource_id")?.as_str()
                .and_then(|s| uuid::Uuid::parse_str(s).ok())
                .unwrap_or_default();
            let desc = r.get("description").and_then(|d| d.as_str()).unwrap_or("").to_string();
            Some(ToolInfo {
                name,
                description: desc,
                resource_id,
                parameters: r.get("capabilities_json").cloned().unwrap_or(serde_json::json!({})),
            })
        }).collect();

        Ok(tools)
    }
}