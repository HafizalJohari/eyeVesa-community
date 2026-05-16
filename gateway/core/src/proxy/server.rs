use crate::proxy::ProxyState;
use hyper::body::Incoming;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Request, Response};
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::net::TcpListener;

pub async fn run(addr: SocketAddr, state: Arc<ProxyState>) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("Proxy server bound to {}", addr);

    loop {
        let (stream, remote_addr) = listener.accept().await?;
        tracing::debug!("Accepted connection from {}", remote_addr);

        let state = state.clone();
        tokio::spawn(async move {
            let service = service_fn(move |req| handle_request(req, state.clone()));
            if let Err(e) = http1::Builder::new()
                .serve_connection(hyper_util::rt::TokioIo::new(stream), service)
                .await
            {
                tracing::error!("Error serving connection from {}: {}", remote_addr, e);
            }
        });
    }
}

pub async fn handle_request(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path().to_string();
    let method = req.method().clone();

    tracing::info!("{} {}", method, path);

    match (method.as_str(), path.as_str()) {
        ("POST", "/v1/mcp") => {
            crate::proxy::mcp_handler::handle_mcp_request(req, state).await
        }
        ("POST", "/v1/register") => {
            crate::proxy::agent_handler::handle_register(req, state).await
        }
        ("POST", "/v1/auth") => {
            crate::proxy::agent_handler::handle_authorize(req, state).await
        }
        ("GET", "/health") => Ok(Response::builder()
            .status(200)
            .body("ok".to_string())?),
        // PTV routes
        ("POST", p) if p.starts_with("/v1/ptv/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        // HITL routes
        ("POST", p) if p.starts_with("/v1/hitl/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("GET", p) if p.starts_with("/v1/hitl/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        // Trust score
        ("GET", p) if p.starts_with("/v1/agents/") && p.ends_with("/trust") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        // Delegation
        ("POST", p) if p.starts_with("/v1/agents/") && p.contains("/delegate") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        // Audit
        ("GET", p) if p.starts_with("/v1/audit") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        // Catch-all forward for other v1 routes
        (_, p) if p.starts_with("/v1/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        _ => Ok(Response::builder()
            .status(404)
            .body("not found".to_string())?),
    }
}