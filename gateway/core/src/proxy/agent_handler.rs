use crate::grpc::ControlPlaneClient;
use crate::proxy::ProxyState;
use http_body_util::BodyExt;
use hyper::body::Incoming;
use hyper::{Request, Response};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

#[derive(Debug, Deserialize)]
struct RegisterAgentReq {
    name: String,
    owner: String,
    #[serde(default)]
    capabilities: Vec<String>,
    #[serde(default)]
    allowed_tools: Vec<String>,
    #[serde(default)]
    max_budget_usd: f64,
}

#[derive(Debug, Serialize)]
struct RegisterAgentResp {
    agent_id: String,
    public_key: String,
    status: String,
    trust_score: f64,
}

#[derive(Debug, Deserialize)]
struct AuthorizeReq {
    agent_id: String,
    action: String,
    #[serde(default)]
    resource_id: String,
}

#[derive(Debug, Serialize)]
struct AuthorizeResp {
    allowed: bool,
    requires_hitl: bool,
    reason: String,
    trust_delta: f64,
}

pub async fn handle_register(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let body = req.into_body();
    let bytes = body.collect().await?.to_bytes();
    let reg: RegisterAgentReq = match serde_json::from_slice(&bytes) {
        Ok(r) => r,
        Err(e) => {
            return Ok(Response::builder()
                .status(400)
                .header("content-type", "application/json")
                .body(format!("{{\"error\": \"{}\"}}", e))?)
        }
    };

    let mut guard = state.control_plane.lock().await;

    if guard.is_none() {
        match ControlPlaneClient::connect(&state.control_plane_addr).await {
            Ok(client) => {
                *guard = Some(client);
                tracing::info!("Connected to control plane at {}", state.control_plane_addr);
            }
            Err(e) => {
                return Ok(Response::builder()
                    .status(502)
                    .body(format!("{{\"error\": \"control plane unavailable: {}\"}}", e))?)
            }
        }
    }

    let client = guard.as_mut().ok_or("No control plane client")?;

    let response = client
        .register_agent(
            reg.name,
            reg.owner,
            reg.capabilities,
            reg.allowed_tools,
            reg.max_budget_usd,
        )
        .await
        .map_err(|e| format!("gRPC error: {}", e))?;

    let resp = RegisterAgentResp {
        agent_id: response.agent_id,
        public_key: base64_encode(&response.public_key),
        status: response.status,
        trust_score: response.trust_score,
    };

    Ok(Response::builder()
        .status(201)
        .header("content-type", "application/json")
        .body(serde_json::to_string(&resp)?)?)
}

pub async fn handle_authorize(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let body = req.into_body();
    let bytes = body.collect().await?.to_bytes();
    let auth_req: AuthorizeReq = match serde_json::from_slice(&bytes) {
        Ok(r) => r,
        Err(e) => {
            return Ok(Response::builder()
                .status(400)
                .body(format!("{{\"error\": \"{}\"}}", e))?)
        }
    };

    let mut guard = state.control_plane.lock().await;

    if guard.is_none() {
        match ControlPlaneClient::connect(&state.control_plane_addr).await {
            Ok(client) => {
                *guard = Some(client);
            }
            Err(e) => {
                return Ok(Response::builder()
                    .status(502)
                    .body(format!("{{\"error\": \"control plane unavailable: {}\"}}", e))?)
            }
        }
    }

    let client = guard.as_mut().ok_or("No control plane client")?;

    let response = client
        .authorize(
            auth_req.agent_id,
            auth_req.resource_id,
            auth_req.action,
            "{}".to_string(),
        )
        .await
        .map_err(|e| format!("gRPC error: {}", e))?;

    let resp = AuthorizeResp {
        allowed: response.allowed,
        requires_hitl: response.requires_hitl,
        reason: response.reason,
        trust_delta: response.trust_delta,
    };

    Ok(Response::builder()
        .status(200)
        .header("content-type", "application/json")
        .body(serde_json::to_string(&resp)?)?)
}

fn base64_encode(data: &[u8]) -> String {
    base64::Engine::encode(&base64::engine::general_purpose::STANDARD, data)
}