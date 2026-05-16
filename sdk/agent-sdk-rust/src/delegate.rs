use crate::client::AgentClient;

#[derive(Debug, thiserror::Error)]
pub enum DelegateError {
    #[error("Delegation not allowed: {0}")]
    NotAllowed(String),
    #[error("Max depth exceeded: {0}")]
    MaxDepth(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
}

impl AgentClient {
    pub async fn delegate(
        &self,
        child_agent_id: &uuid::Uuid,
        scope: Vec<String>,
        max_depth: u32,
    ) -> Result<(), DelegateError> {
        tracing::info!(
            "Delegating to agent {} with scope {:?}",
            child_agent_id,
            scope
        );

        Ok(())
    }
}