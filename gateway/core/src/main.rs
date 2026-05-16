mod crypto;
mod grpc;
mod identity;
mod proxy;
mod tls;

pub mod proto {
    tonic::include_proto!("agentid");
}

use proxy::ProxyState;
use std::sync::Arc;
use tokio::sync::Mutex;
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env().add_directive("info".parse()?))
        .init();

    tracing::info!("AgentID Core Gateway starting...");

    rustls::crypto::ring::default_provider()
        .install_default()
        .expect("Failed to install rustls crypto provider");

    let control_plane_addr = std::env::var("CONTROL_PLANE_ADDR")
        .unwrap_or_else(|_| "http://localhost:9090".to_string());
    let control_plane_http_addr = std::env::var("CONTROL_PLANE_HTTP_ADDR")
        .unwrap_or_else(|_| "localhost:8080".to_string());
    let mode = std::env::var("GATEWAY_MODE")
        .unwrap_or_else(|_| "plaintext".to_string());
    tracing::info!("Control plane gRPC address: {}", control_plane_addr);
    tracing::info!("Control plane HTTP address: {}", control_plane_http_addr);
    tracing::info!("Gateway mode: {}", mode);

    match identity::svid::fetch_identity(&control_plane_http_addr).await {
        Ok(id) => tracing::info!(
            "Gateway identity: {} (trust domain: {}, expires: {})",
            id.spiffe_id, id.trust_domain, id.expires_at
        ),
        Err(e) => tracing::warn!("Could not fetch gateway identity: {}", e),
    }

    let state = Arc::new(ProxyState {
        control_plane: Arc::new(Mutex::new(None)),
        control_plane_addr: control_plane_addr.clone(),
        control_plane_http_addr: control_plane_http_addr.clone(),
        http_client: reqwest::Client::new(),
    });

    match grpc::ControlPlaneClient::connect(&control_plane_addr).await {
        Ok(client) => {
            tracing::info!("Connected to control plane at {}", control_plane_addr);
            let mut guard = state.control_plane.lock().await;
            *guard = Some(client);
        }
        Err(e) => {
            tracing::warn!("Control plane not available yet: {} (will connect on first request)", e);
        }
    }

    let cert_watcher_handle = if mode == "tls" || mode == "mtls" {
        let tls_config = tls::TlsConfig::from_env();
        let watcher = Arc::new(tls::watcher::CertWatcher::new(tls_config));
        let w = watcher.clone();
        tokio::spawn(async move {
            w.watch_loop().await;
        });
        Some(watcher.receiver())
    } else {
        None
    };

    match mode.as_str() {
        "tls" => {
            let tls_config = tls::TlsConfig::from_env();
            let addr = std::net::SocketAddr::from(([0, 0, 0, 0], 9443));
            tracing::info!("Starting TLS proxy on {}", addr);
            tls::server::run_tls(addr, &tls_config, state).await?;
        }
        "mtls" => {
            let tls_config = tls::TlsConfig::from_env();
            let addr = std::net::SocketAddr::from(([0, 0, 0, 0], 9443));
            tracing::info!("Starting mTLS proxy on {}", addr);
            tls::server::run_mtls(addr, &tls_config, state).await?;
        }
        _ => {
            let addr = std::net::SocketAddr::from(([0, 0, 0, 0], 9443));
            tracing::info!("Proxy server listening on {} (plaintext)", addr);
            proxy::server::run(addr, state).await?;
        }
    }

    drop(cert_watcher_handle);

    Ok(())
}