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
                "prompts": { "listChanged": true }
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