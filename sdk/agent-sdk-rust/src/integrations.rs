use crate::client::AgentClient;
use crate::airport::AirportAgent;

#[derive(Debug, thiserror::Error)]
pub enum IntegrationError {
    #[error("Integration error: {0}")]
    Client(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
    #[error("Not authorized: {0}")]
    NotAuthorized(String),
    #[error("HITL required: {0}")]
    HitlRequired(String),
}

static EYEVESA_TOOL_DEFINITIONS: &[(&str, &str)] = &[
    ("eyevesa_read", "Read data from an eyeVesa-gated resource. Authorization is checked via OPA policy. High-risk reads may require HITL approval."),
    ("eyevesa_write", "Write data to an eyeVesa-gated resource. Writes are typically higher risk and may require HITL approval."),
    ("eyevesa_request_approval", "Proactively request human-in-the-loop approval for an action. Use for sensitive operations like bank transfers or data deletion."),
    ("eyevesa_discover", "Discover available resources registered with the eyeVesa gateway."),
    ("eyevesa_delegate", "Delegate scoped permissions to another agent. Maximum delegation depth is 3."),
    ("eyevesa_skill_trust", "Check per-skill trust scores for an agent. Use to assess whether an agent has sufficient trust for a particular skill."),
];

pub struct HermesIntegration {
    client: AgentClient,
    heartbeat_status: String,
}

impl HermesIntegration {
    pub fn new(client: AgentClient) -> Self {
        Self {
            client,
            heartbeat_status: "idle".to_string(),
        }
    }

    pub fn get_tool_specs(&self) -> Vec<serde_json::Value> {
        EYEVESA_TOOL_DEFINITIONS
            .iter()
            .map(|(name, desc)| {
                serde_json::json!({
                    "name": name,
                    "description": desc,
                    "action_type": "eyevesa_gateway",
                })
            })
            .collect()
    }

    pub async fn handle_action(&self, action_name: &str, action_input: serde_json::Value) -> Result<serde_json::Value, IntegrationError> {
        match action_name {
            "eyevesa_read" => {
                let resource_id_str = action_input["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = action_input.get("query").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "read", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_write" => {
                let resource_id_str = action_input["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = action_input.get("data").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "write", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_request_approval" => {
                let action = action_input["action"].as_str().unwrap_or("");
                let reason = action_input["reason"].as_str().unwrap_or("");
                let risk_level = action_input["risk_level"].as_str().unwrap_or("medium");
                let result = self.client.request_approval(action, reason, risk_level).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"approval_id": result.approval_id, "status": result.status}))
            }
            "eyevesa_discover" => {
                let capability = action_input.get("capability").and_then(|v| v.as_str()).unwrap_or("mcp");
                let result = self.client.discover(capability).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            "eyevesa_delegate" => {
                let delegatee_id_str = action_input["delegatee_id"].as_str().unwrap_or("");
                let delegatee_id = uuid::Uuid::parse_str(delegatee_id_str).unwrap_or_default();
                let scope: Vec<String> = action_input["scope"].as_array()
                    .map(|a| a.iter().filter_map(|v| v.as_str().map(String::from)).collect())
                    .unwrap_or_default();
                let reason = action_input.get("reason").and_then(|v| v.as_str()).unwrap_or("");
                let result = self.client.delegate(&delegatee_id, scope, reason).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"delegation_id": result.delegation_id, "status": result.status}))
            }
            "eyevesa_skill_trust" => {
                let agent_id = action_input["agent_id"].as_str().unwrap_or("");
                let result = self.client.get_skill_trust(agent_id).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            _ => Err(IntegrationError::Client(format!("Unknown action: {}", action_name))),
        }
    }

    pub async fn heartbeat(&mut self, status: &str) -> Result<serde_json::Value, IntegrationError> {
        self.heartbeat_status = status.to_string();
        self.client.airport_heartbeat(status).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn discover_peers(&self, params: &[(&str, &str)]) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_search(params).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn list_online_peers(&self) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_list_online().await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub fn heartbeat_status(&self) -> &str {
        &self.heartbeat_status
    }

    pub fn client(&self) -> &AgentClient {
        &self.client
    }
}

pub struct OpenClawIntegration {
    client: AgentClient,
}

impl OpenClawIntegration {
    pub fn new(client: AgentClient) -> Self {
        Self { client }
    }

    pub fn get_tool_specs(&self) -> Vec<serde_json::Value> {
        EYEVESA_TOOL_DEFINITIONS
            .iter()
            .map(|(name, desc)| {
                serde_json::json!({
                    "name": name,
                    "description": desc,
                    "handler": "eyevesa_gateway",
                    "source": "eyevesa",
                    "permissions": ["read", "write"],
                })
            })
            .collect()
    }

    pub async fn execute_tool(&self, tool_name: &str, arguments: serde_json::Value) -> Result<serde_json::Value, IntegrationError> {
        match tool_name {
            "eyevesa_read" => {
                let resource_id_str = arguments["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = arguments.get("query").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "read", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_write" => {
                let resource_id_str = arguments["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = arguments.get("data").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "write", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_request_approval" => {
                let action = arguments["action"].as_str().unwrap_or("");
                let reason = arguments["reason"].as_str().unwrap_or("");
                let risk_level = arguments["risk_level"].as_str().unwrap_or("medium");
                let result = self.client.request_approval(action, reason, risk_level).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"approval_id": result.approval_id, "status": result.status}))
            }
            "eyevesa_discover" => {
                let capability = arguments.get("capability").and_then(|v| v.as_str()).unwrap_or("mcp");
                let result = self.client.discover(capability).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            "eyevesa_delegate" => {
                let delegatee_id_str = arguments["delegatee_id"].as_str().unwrap_or("");
                let delegatee_id = uuid::Uuid::parse_str(delegatee_id_str).unwrap_or_default();
                let scope: Vec<String> = arguments["scope"].as_array()
                    .map(|a| a.iter().filter_map(|v| v.as_str().map(String::from)).collect())
                    .unwrap_or_default();
                let reason = arguments.get("reason").and_then(|v| v.as_str()).unwrap_or("");
                let result = self.client.delegate(&delegatee_id, scope, reason).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"delegation_id": result.delegation_id, "status": result.status}))
            }
            "eyevesa_skill_trust" => {
                let agent_id = arguments["agent_id"].as_str().unwrap_or("");
                let result = self.client.get_skill_trust(agent_id).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            _ => Err(IntegrationError::Client(format!("Unknown tool: {}", tool_name))),
        }
    }

    pub async fn register_at_airport(&self, description: &str, tags: Vec<&str>, listed: bool) -> Result<serde_json::Value, IntegrationError> {
        self.client.airport_heartbeat("online").await
            .map_err(|e| IntegrationError::Client(e.to_string()))?;
        let update = serde_json::json!({
            "description": description,
            "tags": tags,
            "listed": listed,
        });
        self.client.airport_update_profile(update).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn discover_agents(&self, params: &[(&str, &str)]) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_search(params).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn list_online_agents(&self) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_list_online().await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub fn client(&self) -> &AgentClient {
        &self.client
    }
}

pub struct NanoClawIntegration {
    client: AgentClient,
}

impl NanoClawIntegration {
    pub fn new(client: AgentClient) -> Self {
        Self { client }
    }

    pub fn get_function_definitions(&self) -> Vec<serde_json::Value> {
        EYEVESA_TOOL_DEFINITIONS
            .iter()
            .map(|(name, desc)| {
                serde_json::json!({
                    "name": name,
                    "description": desc,
                    "guardrails": { "input_validation": true, "output_validation": true },
                    "trust_requirement": if name.contains("read") { 0.5 } else { 0.7 },
                })
            })
            .collect()
    }

    pub async fn execute_function(&self, function_name: &str, arguments: serde_json::Value) -> Result<serde_json::Value, IntegrationError> {
        match function_name {
            "eyevesa_read" => {
                let resource_id_str = arguments["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = arguments.get("query").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "read", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_write" => {
                let resource_id_str = arguments["resource_id"].as_str().unwrap_or("");
                let resource_id = uuid::Uuid::parse_str(resource_id_str).unwrap_or_default();
                let params = arguments.get("data").cloned().unwrap_or(serde_json::json!({}));
                let result = self.client.invoke(&resource_id, "write", params).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"success": result.success, "data": result.data, "trust_score": result.trust_score}))
            }
            "eyevesa_request_approval" => {
                let action = arguments["action"].as_str().unwrap_or("");
                let reason = arguments["reason"].as_str().unwrap_or("");
                let risk_level = arguments["risk_level"].as_str().unwrap_or("medium");
                let result = self.client.request_approval(action, reason, risk_level).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"approval_id": result.approval_id, "status": result.status}))
            }
            "eyevesa_discover" => {
                let capability = arguments.get("capability").and_then(|v| v.as_str()).unwrap_or("mcp");
                let result = self.client.discover(capability).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            "eyevesa_delegate" => {
                let delegatee_id_str = arguments["delegatee_id"].as_str().unwrap_or("");
                let delegatee_id = uuid::Uuid::parse_str(delegatee_id_str).unwrap_or_default();
                let scope: Vec<String> = arguments["scope"].as_array()
                    .map(|a| a.iter().filter_map(|v| v.as_str().map(String::from)).collect())
                    .unwrap_or_default();
                let reason = arguments.get("reason").and_then(|v| v.as_str()).unwrap_or("");
                let result = self.client.delegate(&delegatee_id, scope, reason).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!({"delegation_id": result.delegation_id, "status": result.status}))
            }
            "eyevesa_skill_trust" => {
                let agent_id = arguments["agent_id"].as_str().unwrap_or("");
                let result = self.client.get_skill_trust(agent_id).await
                    .map_err(|e| IntegrationError::Client(e.to_string()))?;
                Ok(serde_json::json!(result))
            }
            _ => Err(IntegrationError::Client(format!("Unknown function: {}", function_name))),
        }
    }

    pub async fn check_trust(&self, agent_id: &str, min_trust: f64) -> Result<bool, IntegrationError> {
        let profile = self.client.airport_get_profile(agent_id).await
            .map_err(|e| IntegrationError::Client(e.to_string()))?;
        Ok(profile.trust_score >= min_trust)
    }

    pub async fn heartbeat(&self, status: &str) -> Result<serde_json::Value, IntegrationError> {
        self.client.airport_heartbeat(status).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn discover_agents(&self, params: &[(&str, &str)]) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_search(params).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn list_online_agents(&self) -> Result<Vec<AirportAgent>, IntegrationError> {
        self.client.airport_list_online().await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub async fn get_agent_profile(&self, agent_id: &str) -> Result<AirportAgent, IntegrationError> {
        self.client.airport_get_profile(agent_id).await
            .map_err(|e| IntegrationError::Client(e.to_string()))
    }

    pub fn client(&self) -> &AgentClient {
        &self.client
    }
}