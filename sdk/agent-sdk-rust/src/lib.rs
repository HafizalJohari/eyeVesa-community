pub mod client;
pub mod connect;
pub mod discover;
pub mod invoke;
pub mod delegate;

use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentConfig {
    pub agent_id: Uuid,
    pub name: String,
    pub owner: String,
    pub gateway_endpoint: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolInfo {
    pub name: String,
    pub description: String,
    pub resource_id: Uuid,
    pub parameters: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvokeResult {
    pub success: bool,
    pub data: serde_json::Value,
    pub trust_score: f64,
}