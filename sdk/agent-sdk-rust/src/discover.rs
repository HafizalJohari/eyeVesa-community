use crate::client::AgentClient;
use crate::ToolInfo;

#[derive(Debug, thiserror::Error)]
pub enum DiscoverError {
    #[error("No resources found matching: {0}")]
    NotFound(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
}

impl AgentClient {
    pub async fn discover(&self, capability: &str) -> Result<Vec<ToolInfo>, DiscoverError> {
        tracing::info!("Discovering tools for capability: {}", capability);

        Ok(vec![])
    }
}