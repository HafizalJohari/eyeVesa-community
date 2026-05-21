use crate::client::AgentClient;

#[derive(Debug, thiserror::Error)]
pub enum McpError {
    #[error("MCP request failed: {0}")]
    RequestFailed(String),
    #[error("Method not found: {0}")]
    MethodNotFound(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(Debug, Clone)]
pub struct McpCapabilities {
    pub protocol_version: String,
    pub tools: bool,
    pub resources: bool,
    pub prompts: bool,
}

#[derive(Debug, Clone)]
pub struct McpTool {
    pub name: String,
    pub description: Option<String>,
    pub input_schema: Option<serde_json::Value>,
}

impl AgentClient {
    pub async fn mcp_initialize(&self) -> Result<McpCapabilities, McpError> {
        tracing::info!("Initializing MCP connection");

        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/mcp", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "jsonrpc": "2.0",
            "method": "initialize",
            "id": 1,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(McpError::RequestFailed(format!("status: {}", resp.status())));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| McpError::RequestFailed(e.to_string()))?;

        let result_data = result.get("result").cloned().unwrap_or(serde_json::json!({}));

        let caps = result_data.get("capabilities").cloned().unwrap_or(serde_json::json!({}));
        let proto_version = result_data.get("protocolVersion")
            .and_then(|v| v.as_str())
            .unwrap_or("unknown")
            .to_string();

        Ok(McpCapabilities {
            protocol_version: proto_version,
            tools: caps.get("tools").is_some(),
            resources: caps.get("resources").is_some(),
            prompts: caps.get("prompts").is_some(),
        })
    }

    pub async fn mcp_list_tools(&self) -> Result<Vec<McpTool>, McpError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/mcp", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "jsonrpc": "2.0",
            "method": "tools/list",
            "id": 2,
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(McpError::RequestFailed(format!("status: {}", resp.status())));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| McpError::RequestFailed(e.to_string()))?;

        let result_data = result.get("result").cloned().unwrap_or(serde_json::json!({}));
        let tools_arr = result_data.get("tools")
            .and_then(|t| t.as_array())
            .cloned()
            .unwrap_or_default();

        let tools: Vec<McpTool> = tools_arr.iter().filter_map(|t| {
            Some(McpTool {
                name: t.get("name")?.as_str()?.to_string(),
                description: t.get("description").and_then(|d| d.as_str()).map(String::from),
                input_schema: t.get("inputSchema").cloned(),
            })
        }).collect();

        Ok(tools)
    }

    pub async fn mcp_call_tool(
        &self,
        tool_name: &str,
        arguments: serde_json::Value,
    ) -> Result<serde_json::Value, McpError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/mcp", gateway.trim_end_matches('/'));

        let body = serde_json::json!({
            "jsonrpc": "2.0",
            "method": "tools/call",
            "id": 3,
            "params": {
                "name": tool_name,
                "arguments": arguments,
            }
        });

        let resp = self.http_client()
            .post(&url)
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            return Err(McpError::RequestFailed(format!("status: {}", resp.status())));
        }

        let result: serde_json::Value = resp.json().await
            .map_err(|e| McpError::RequestFailed(e.to_string()))?;

        Ok(result.get("result").cloned().unwrap_or(serde_json::json!({})))
    }
}