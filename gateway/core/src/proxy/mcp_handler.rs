use http_body_util::BodyExt;
use hyper::body::Incoming;
use hyper::{Request, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Deserialize)]
struct JsonRpcRequest {
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
        "tools/list" => serde_json::json!({
            "tools": []
        }),
        "resources/list" => serde_json::json!({
            "resources": []
        }),
        "prompts/list" => serde_json::json!({
            "prompts": []
        }),
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