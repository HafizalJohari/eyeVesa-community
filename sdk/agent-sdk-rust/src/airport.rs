use crate::client::AgentClient;

#[derive(Debug, thiserror::Error)]
pub enum AirportError {
    #[error("Airport request failed: {0}")]
    Gateway(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(Debug, Clone, serde::Deserialize)]
pub struct AirportAgent {
    pub agent_id: String,
    pub name: String,
    pub owner: String,
    pub trust_score: f64,
    pub status: String,
    #[serde(default)]
    pub description: String,
    pub services_offered: serde_json::Value,
    pub endpoints: serde_json::Value,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default)]
    pub total_actions: i64,
    #[serde(default)]
    pub approval_rate: f64,
    #[serde(default)]
    pub last_seen: String,
}

#[derive(Debug, Clone, serde::Deserialize)]
pub struct AirportConnection {
    pub connection_id: String,
    pub requester_id: String,
    pub responder_id: String,
    pub action: String,
    pub outcome: String,
    pub trust_score_at_time: f64,
    pub created_at: String,
}

impl AgentClient {
    pub async fn airport_heartbeat(&self, status: &str) -> Result<serde_json::Value, AirportError> {
        let url = format!("{}/v1/airport/heartbeat", self.gateway_endpoint().trim_end_matches('/'));
        let body = serde_json::json!({
            "agent_id": self.agent_id().to_string(),
            "status": status,
        });
        let resp = self.http_client().post(&url).headers(self.auth_headers()).json(&body).send().await?;
        if !resp.status().is_success() {
            return Err(AirportError::Gateway(format!("heartbeat failed: {}", resp.status())));
        }
        Ok(resp.json().await?)
    }

    pub async fn airport_update_profile(&self, update: serde_json::Value) -> Result<serde_json::Value, AirportError> {
        let url = format!("{}/v1/airport/agents/{}", self.gateway_endpoint().trim_end_matches('/'), self.agent_id());
        let resp = self.http_client().put(&url).headers(self.auth_headers()).json(&update).send().await?;
        if !resp.status().is_success() {
            return Err(AirportError::Gateway(format!("profile update failed: {}", resp.status())));
        }
        Ok(resp.json().await?)
    }

    pub async fn airport_search(&self, params: &[(&str, &str)]) -> Result<Vec<AirportAgent>, AirportError> {
        let url = format!("{}/v1/airport/agents", self.gateway_endpoint().trim_end_matches('/'));
        let resp = self.http_client().get(&url).headers(self.auth_headers()).query(params).send().await?;
        if !resp.status().is_success() {
            return Err(AirportError::Gateway(format!("search failed: {}", resp.status())));
        }
        let body: serde_json::Value = resp.json().await?;
        let agents = body.get("agents")
            .and_then(|a| a.as_array())
            .cloned()
            .unwrap_or_default();
        let result: Vec<AirportAgent> = agents.iter()
            .filter_map(|a| serde_json::from_value(a.clone()).ok())
            .collect();
        Ok(result)
    }

    pub async fn airport_get_profile(&self, agent_id: &str) -> Result<AirportAgent, AirportError> {
        let url = format!("{}/v1/airport/agents/{}", self.gateway_endpoint().trim_end_matches('/'), agent_id);
        let resp = self.http_client().get(&url).headers(self.auth_headers()).send().await?;
        let agent: AirportAgent = resp.json().await?;
        Ok(agent)
    }

    pub async fn airport_list_online(&self) -> Result<Vec<AirportAgent>, AirportError> {
        let url = format!("{}/v1/airport/online", self.gateway_endpoint().trim_end_matches('/'));
        let resp = self.http_client().get(&url).headers(self.auth_headers()).send().await?;
        if !resp.status().is_success() {
            return Err(AirportError::Gateway(format!("online list failed: {}", resp.status())));
        }
        let body: serde_json::Value = resp.json().await?;
        let agents = body.get("agents")
            .and_then(|a| a.as_array())
            .cloned()
            .unwrap_or_default();
        let result: Vec<AirportAgent> = agents.iter()
            .filter_map(|a| serde_json::from_value(a.clone()).ok())
            .collect();
        Ok(result)
    }

    pub async fn airport_connections(&self, agent_id: &str, limit: u32) -> Result<Vec<AirportConnection>, AirportError> {
        let url = format!("{}/v1/airport/connections", self.gateway_endpoint().trim_end_matches('/'));
        let resp = self.http_client().get(&url)
            .headers(self.auth_headers())
            .query(&[("agent_id", agent_id), ("limit", &limit.to_string())])
            .send()
            .await?;
        if !resp.status().is_success() {
            return Err(AirportError::Gateway(format!("connections query failed: {}", resp.status())));
        }
        let body: serde_json::Value = resp.json().await?;
        let connections = body.get("connections")
            .and_then(|c| c.as_array())
            .cloned()
            .unwrap_or_default();
        let result: Vec<AirportConnection> = connections.iter()
            .filter_map(|c| serde_json::from_value(c.clone()).ok())
            .collect();
        Ok(result)
    }
}