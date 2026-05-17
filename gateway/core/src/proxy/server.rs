use crate::proxy::ProxyState;
use hyper::body::Incoming;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Request, Response};
use std::net::SocketAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use tokio::net::TcpListener;
use tokio_util::sync::CancellationToken;

pub static DRAINING: AtomicBool = AtomicBool::new(false);

pub async fn run(addr: SocketAddr, state: Arc<ProxyState>, cancel: CancellationToken) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("Proxy server bound to {}", addr);

    let mut conn_handles: Vec<tokio::task::JoinHandle<()>> = Vec::new();

    loop {
        tokio::select! {
            accept_result = listener.accept() => {
                if DRAINING.load(Ordering::SeqCst) {
                    tracing::info!("Draining: rejecting new connection");
                    if let Ok((stream, _)) = accept_result {
                        drop(stream);
                    }
                    continue;
                }

                let (stream, remote_addr) = accept_result?;
                tracing::debug!("Accepted connection from {}", remote_addr);

                let state = state.clone();
                let cancel_clone = cancel.clone();
                let handle = tokio::spawn(async move {
                    let service = service_fn(move |req| handle_request(req, state.clone()));
                    let conn = http1::Builder::new()
                        .serve_connection(hyper_util::rt::TokioIo::new(stream), service);

                    tokio::select! {
                        result = conn => {
                            if let Err(e) = result {
                                tracing::error!("Error serving connection from {}: {}", remote_addr, e);
                            }
                        }
                        _ = cancel_clone.cancelled() => {
                            tracing::debug!("Connection cancelled for {}", remote_addr);
                        }
                    }
                });
                conn_handles.push(handle);
            }
            _ = cancel.cancelled() => {
                tracing::info!("Shutdown signal received, draining connections...");
                DRAINING.store(true, Ordering::SeqCst);

                drop(listener);

                let drain_timeout = std::env::var("DRAIN_TIMEOUT_SECS")
                    .ok()
                    .and_then(|v| v.parse::<u64>().ok())
                    .unwrap_or(30);

                let deadline = tokio::time::Instant::now() + std::time::Duration::from_secs(drain_timeout);

                loop {
                    conn_handles.retain(|h| !h.is_finished());
                    if conn_handles.is_empty() {
                        break;
                    }
                    if tokio::time::Instant::now() >= deadline {
                        tracing::warn!("Drain timeout reached, {} connections still active", conn_handles.len());
                        break;
                    }
                    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                }

                tracing::info!("All connections drained, shutting down");
                return Ok(());
            }
        }
    }
}

pub async fn handle_request(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path().to_string();
    let method = req.method().clone();

    tracing::info!("{} {}", method, path);

    if DRAINING.load(Ordering::SeqCst) && path != "/health" && path != "/ready" {
        return Ok(Response::builder()
            .status(503)
            .header("content-type", "application/json")
            .body(r#"{"error":"server is draining"}"#.to_string())?);
    }

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
        ("GET", "/ready") => {
            if DRAINING.load(Ordering::SeqCst) {
                Ok(Response::builder()
                    .status(503)
                    .body("draining".to_string())?)
            } else {
                Ok(Response::builder()
                    .status(200)
                    .body("ready".to_string())?)
            }
        }
        ("POST", p) if p.starts_with("/v1/ptv/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("POST", p) if p.starts_with("/v1/hitl/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("GET", p) if p.starts_with("/v1/hitl/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("GET", p) if p.starts_with("/v1/agents/") && p.ends_with("/trust") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("POST", p) if p.starts_with("/v1/agents/") && p.contains("/delegate") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        ("GET", p) if p.starts_with("/v1/audit") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        (_, p) if p.starts_with("/v1/") => {
            crate::proxy::forward::forward_to_control_plane(req, state).await
        }
        _ => Ok(Response::builder()
            .status(404)
            .body("not found".to_string())?),
    }
}