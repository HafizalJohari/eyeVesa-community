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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthorizeResult {
    pub allowed: bool,
    pub requires_hitl: bool,
    pub reason: String,
    pub trust_delta: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DelegateResult {
    pub delegation_id: Uuid,
    pub status: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PtvAttestResult {
    pub attestation: serde_json::Value,
    pub tpm_signature: String,
    pub quote: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PtvBindResult {
    pub binding_id: Uuid,
    pub agent_id: String,
    pub platform: String,
    pub transformed_at: i64,
    pub expires_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HitlApproval {
    pub approval_id: Uuid,
    pub agent_id: String,
    pub action: String,
    pub status: String,
    pub expires_at: Option<String>,
}