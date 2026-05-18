use crate::grpc::ControlPlaneClient;
use crate::proxy::ProxyState;
use http_body_util::BodyExt;
use hyper::body::Incoming;
use hyper::{Request, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::sync::Arc;

#[derive(Debug, Deserialize)]
struct JsonRpcRequest {
    #[allow(dead_code)]
    jsonrpc: String,
    id: Option<Value>,
    method: String,
    params: Option<Value>,
}

#[derive(Debug, Serialize)]
struct JsonRpcResponse {
    jsonrpc: String,
    id: Option<Value>,
    result: Option<Value>,
    error: Option<JsonRpcError>,
}

#[derive(Debug, Serialize)]
struct JsonRpcError {
    code: i32,
    message: String,
}

pub async fn handle_mcp_request(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let body = req.into_body();
    let bytes = body.collect().await?.to_bytes();
    let rpc_req: JsonRpcRequest = match serde_json::from_slice(&bytes) {
        Ok(r) => r,
        Err(e) => {
            let resp = JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id: None,
                result: None,
                error: Some(JsonRpcError {
                    code: -32700,
                    message: format!("Parse error: {}", e),
                }),
            };
            return Ok(Response::builder()
                .status(200)
                .header("content-type", "application/json")
                .body(serde_json::to_string(&resp)?)?);
        }
    };

    tracing::info!("MCP request: method={}, id={:?}", rpc_req.method, rpc_req.id);

    let result = match rpc_req.method.as_str() {
        "initialize" => serde_json::json!({
            "protocolVersion": "2024-11-05",
            "capabilities": {
                "tools": { "listChanged": true },
                "resources": { "subscribe": true },
                "prompts": { "listChanged": true },
                "skills": { "listChanged": true }
            },
            "serverInfo": {
                "name": "agentid-gateway",
                "version": "0.1.0"
            }
        }),
        "tools/list" => {
            let tools = match list_tools_via_grpc(&state).await {
                Ok(t) => t,
                Err(_) => serde_json::json!([]),
            };
            serde_json::json!({ "tools": tools })
        }
        "tools/call" => {
            let params = rpc_req.params.unwrap_or_default();
            let tool_name = params.get("name").and_then(|v| v.as_str()).unwrap_or("");
            let agent_id = params.get("arguments")
                .and_then(|a| a.get("agent_id"))
                .and_then(|v| v.as_str())
                .unwrap_or("");

            if !agent_id.is_empty() && !tool_name.is_empty() {
                match authorize_via_grpc(&state, agent_id, tool_name).await {
                    Ok(authz) => {
                        if authz.allowed {
                            serde_json::json!({
                                "content": [{
                                    "type": "text",
                                    "text": format!("Action '{}' authorized for agent {}", tool_name, agent_id)
                                }],
                                "authorization": authz
                            })
                        } else {
                            serde_json::json!({
                                "isError": true,
                                "content": [{
                                    "type": "text",
                                    "text": format!("Action '{}' denied: {}", tool_name, authz.reason)
                                }],
                                "authorization": authz
                            })
                        }
                    }
                    Err(e) => serde_json::json!({
                        "isError": true,
                        "content": [{"type": "text", "text": format!("Authorization error: {}", e)}]
                    })
                }
            } else {
                serde_json::json!({
                    "isError": true,
                    "content": [{"type": "text", "text": "Missing agent_id or tool name in arguments"}]
                })
            }
        }
        "resources/list" => serde_json::json!({ "resources": [] }),
        "prompts/list" => serde_json::json!({ "prompts": [] }),
        "skills/list" => {
            let url = format!("{}/v1/skills", state.control_plane_http_addr.read().await.clone());
            match state.http_client.get(&url).send().await {
                Ok(resp) => {
                    let body: serde_json::Value = resp.json().await.unwrap_or(serde_json::json!({"skills": []}));
                    body
                }
                Err(_) => serde_json::json!({"skills": []}),
            }
        }
        "skills/search" => {
            let query = rpc_req.params.as_ref()
                .and_then(|p| p.get("query"))
                .and_then(|q| q.as_str())
                .unwrap_or("");
            let category = rpc_req.params.as_ref()
                .and_then(|p| p.get("category"))
                .and_then(|c| c.as_str())
                .unwrap_or("");
            let url = format!("{}/v1/skills/search?q={}&category={}",
                state.control_plane_http_addr.read().await.clone(),
                query, category);
            match state.http_client.get(&url).send().await {
                Ok(resp) => {
                    let body: serde_json::Value = resp.json().await.unwrap_or(serde_json::json!({"skills": []}));
                    body
                }
                Err(_) => serde_json::json!({"skills": []}),
            }
        }
        "skills/endorse" => {
            let url = format!("{}/v1/agents/{}/skills/{}/endorse",
                state.control_plane_http_addr.read().await.clone(),
                rpc_req.params.as_ref().and_then(|p| p.get("agent_id")).and_then(|v| v.as_str()).unwrap_or(""),
                rpc_req.params.as_ref().and_then(|p| p.get("skill_id")).and_then(|v| v.as_str()).unwrap_or(""));
            let body = serde_json::json!({
                "endorser_type": rpc_req.params.as_ref().and_then(|p| p.get("endorser_type")).and_then(|v| v.as_str()).unwrap_or("agent"),
                "endorser_id": rpc_req.params.as_ref().and_then(|p| p.get("endorser_id")).and_then(|v| v.as_str()).unwrap_or(""),
                "comment": rpc_req.params.as_ref().and_then(|p| p.get("comment")).and_then(|v| v.as_str()).unwrap_or(""),
            });
            match state.http_client.post(&url).json(&body).send().await {
                Ok(resp) => {
                    let resp_body: serde_json::Value = resp.json().await.unwrap_or(serde_json::json!({}));
                    resp_body
                }
                Err(_) => serde_json::json!({"error": "failed to endorse skill"}),
            }
        },
        "airport/search" => {
            let base_url = state.control_plane_http_addr.read().await.clone();
            let capability = rpc_req.params.as_ref().and_then(|p| p.get("capability")).and_then(|v| v.as_str()).unwrap_or("");
            let skill = rpc_req.params.as_ref().and_then(|p| p.get("skill")).and_then(|v| v.as_str()).unwrap_or("");
            let min_trust = rpc_req.params.as_ref().and_then(|p| p.get("min_trust")).and_then(|v| v.as_f64()).unwrap_or(0.0);
            let status = rpc_req.params.as_ref().and_then(|p| p.get("status")).and_then(|v| v.as_str()).unwrap_or("");
            let limit = rpc_req.params.as_ref().and_then(|p| p.get("limit")).and_then(|v| v.as_u64()).unwrap_or(50);
            let mut url = format!("{}/v1/airport/agents?min_trust={}&limit={}", base_url, min_trust, limit);
            if !capability.is_empty() { url = format!("{}&capability={}", url, capability); }
            if !skill.is_empty() { url = format!("{}&skill={}", url, skill); }
            if !status.is_empty() { url = format!("{}&status={}", url, status); }
            match state.http_client.get(&url).send().await {
                Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({"agents": []})),
                Err(_) => serde_json::json!({"agents": []}),
            }
        }
        "airport/heartbeat" => {
            let url = format!("{}/v1/airport/heartbeat", state.control_plane_http_addr.read().await.clone());
            let body = serde_json::json!({
                "agent_id": rpc_req.params.as_ref().and_then(|p| p.get("agent_id")).and_then(|v| v.as_str()).unwrap_or(""),
                "status": rpc_req.params.as_ref().and_then(|p| p.get("status")).and_then(|v| v.as_str()).unwrap_or("online"),
                "metadata": rpc_req.params.as_ref().and_then(|p| p.get("metadata")).cloned().unwrap_or(serde_json::json!({})),
            });
            match state.http_client.post(&url).json(&body).send().await {
                Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({})),
                Err(_) => serde_json::json!({"error": "heartbeat failed"}),
            }
        }
        "airport/profile" => {
            let agent_id = rpc_req.params.as_ref().and_then(|p| p.get("agent_id")).and_then(|v| v.as_str()).unwrap_or("");
            let base_url = state.control_plane_http_addr.read().await.clone();
            if rpc_req.method.as_str() == "airport/profile" && rpc_req.params.as_ref().and_then(|p| p.get("update")).is_some() {
                let url = format!("{}/v1/airport/agents/{}", base_url, agent_id);
                let update = rpc_req.params.as_ref().and_then(|p| p.get("update")).cloned().unwrap_or(serde_json::json!({}));
                match state.http_client.put(&url).json(&update).send().await {
                    Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({})),
                    Err(_) => serde_json::json!({"error": "profile update failed"}),
                }
            } else {
                let url = format!("{}/v1/airport/agents/{}", base_url, agent_id);
                match state.http_client.get(&url).send().await {
                    Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({})),
                    Err(_) => serde_json::json!({"error": "agent not found"}),
                }
            }
        }
        "airport/online" => {
            let url = format!("{}/v1/airport/online", state.control_plane_http_addr.read().await.clone());
            match state.http_client.get(&url).send().await {
                Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({"agents": []})),
                Err(_) => serde_json::json!({"agents": []}),
            }
        }
        "airport/connections" => {
            let agent_id = rpc_req.params.as_ref().and_then(|p| p.get("agent_id")).and_then(|v| v.as_str()).unwrap_or("");
            let limit = rpc_req.params.as_ref().and_then(|p| p.get("limit")).and_then(|v| v.as_u64()).unwrap_or(50);
            let url = format!("{}/v1/airport/connections?agent_id={}&limit={}",
                state.control_plane_http_addr.read().await.clone(), agent_id, limit);
            match state.http_client.get(&url).send().await {
                Ok(resp) => resp.json().await.unwrap_or(serde_json::json!({"connections": []})),
                Err(_) => serde_json::json!({"connections": []}),
            }
        }
        _ => {
            let resp = JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id: rpc_req.id,
                result: None,
                error: Some(JsonRpcError {
                    code: -32601,
                    message: format!("Method not found: {}", rpc_req.method),
                }),
            };
            return Ok(Response::builder()
                .status(200)
                .header("content-type", "application/json")
                .body(serde_json::to_string(&resp)?)?);
        }
    };

    let resp = JsonRpcResponse {
        jsonrpc: "2.0".to_string(),
        id: rpc_req.id,
        result: Some(result),
        error: None,
    };

    Ok(Response::builder()
        .status(200)
        .header("content-type", "application/json")
        .body(serde_json::to_string(&resp)?)?)
}

#[derive(Debug, Clone, Serialize)]
struct AuthzResult {
    allowed: bool,
    requires_hitl: bool,
    reason: String,
    trust_delta: f64,
}

async fn authorize_via_grpc(
    state: &Arc<ProxyState>,
    agent_id: &str,
    action: &str,
) -> Result<AuthzResult, String> {
    let mut guard = state.control_plane.lock().await;

    if guard.is_none() {
        match ControlPlaneClient::connect(&state.control_plane_addr).await {
            Ok(client) => {
                *guard = Some(client);
                tracing::info!("Connected to control plane at {}", state.control_plane_addr);
            }
            Err(e) => {
                return Err(format!("Failed to connect to control plane: {}", e));
            }
        }
    }

    let client = guard.as_mut().ok_or("No control plane client")?;

    let response = client
        .authorize(agent_id.to_string(), String::new(), action.to_string(), "{}".to_string())
        .await
        .map_err(|e| format!("gRPC authorize error: {}", e))?;

    Ok(AuthzResult {
        allowed: response.allowed,
        requires_hitl: response.requires_hitl,
        reason: response.reason,
        trust_delta: response.trust_delta,
    })
}

async fn list_tools_via_grpc(
    state: &Arc<ProxyState>,
) -> Result<serde_json::Value, String> {
    let mut guard = state.control_plane.lock().await;

    if guard.is_none() {
        match ControlPlaneClient::connect(&state.control_plane_addr).await {
            Ok(client) => {
                *guard = Some(client);
            }
            Err(e) => {
                return Err(format!("Failed to connect to control plane: {}", e));
            }
        }
    }

    Ok(serde_json::json!([]))
}