use agentid_core::proxy::ProxyState;
use agentid_core::tls;
use agentid_core::identity;
use agentid_core::grpc;
use agentid_core::proxy;
use std::sync::Arc;
use tokio::sync::Mutex;
use tokio_util::sync::CancellationToken;
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
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
    let listen_port: u16 = std::env::var("PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(9443);
    let backend_tls = tls::BackendTlsConfig::from_env();
    tracing::info!("Control plane gRPC address: {}", control_plane_addr);
    tracing::info!("Control plane HTTP address: {}", control_plane_http_addr);
    tracing::info!("Gateway mode: {}", mode);
    tracing::info!("Backend TLS enabled: {}", backend_tls.enabled);

    match identity::svid::fetch_identity(&control_plane_http_addr, backend_tls.enabled).await {
        Ok(id) => tracing::info!(
            "Gateway identity: {} (trust domain: {}, expires: {})",
            id.spiffe_id, id.trust_domain, id.expires_at
        ),
        Err(e) => tracing::warn!("Could not fetch gateway identity: {}", e),
    }

    let http_client = backend_tls.build_reqwest_client()
        .unwrap_or_else(|e| {
            tracing::warn!("Failed to build TLS client, falling back to default: {}", e);
            reqwest::Client::new()
        });

    let grpc_addr = if backend_tls.enabled {
        let addr = control_plane_addr.trim_start_matches("http://").trim_start_matches("https://");
        format!("https://{}", addr)
    } else {
        control_plane_addr.clone()
    };

    let state = Arc::new(ProxyState {
        control_plane: Arc::new(Mutex::new(None)),
        control_plane_addr: grpc_addr.clone(),
        control_plane_http_addr: Arc::new(tokio::sync::RwLock::new(control_plane_http_addr.clone())),
        http_client,
        backend_tls,
    });

    match grpc::ControlPlaneClient::connect(&grpc_addr).await {
        Ok(client) => {
            tracing::info!("Connected to control plane at {}", grpc_addr);
            let mut guard = state.control_plane.lock().await;
            *guard = Some(client);
        }
        Err(e) => {
            tracing::warn!("Control plane not available yet: {} (will connect on first request)", e);
        }
    }

    let cancel = CancellationToken::new();
    let cancel_clone = cancel.clone();

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

    let server_state = state.clone();
    let server_handle = tokio::spawn(async move {
        match mode.as_str() {
            "tls" => {
                let tls_config = tls::TlsConfig::from_env();
                let addr = std::net::SocketAddr::from(([0, 0, 0, 0], listen_port));
                tracing::info!("Starting TLS proxy on {}", addr);
                tls::server::run_tls(addr, &tls_config, server_state, cancel_clone).await
            }
            "mtls" => {
                let tls_config = tls::TlsConfig::from_env();
                let addr = std::net::SocketAddr::from(([0, 0, 0, 0], listen_port));
                tracing::info!("Starting mTLS proxy on {}", addr);
                tls::server::run_mtls(addr, &tls_config, server_state, cancel_clone).await
            }
            _ => {
                let addr = std::net::SocketAddr::from(([0, 0, 0, 0], listen_port));
                tracing::info!("Proxy server listening on {} (plaintext)", addr);
                proxy::server::run(addr, server_state, cancel_clone).await
            }
        }
    });

    let cancel_sigint = cancel.clone();
    tokio::spawn(async move {
        tokio::signal::ctrl_c().await.expect("failed to listen for ctrl_c");
        tracing::info!("Received SIGINT, initiating graceful shutdown...");
        cancel_sigint.cancel();
    });

    let cancel_sigterm = cancel.clone();
    tokio::spawn(async move {
        let mut sigterm = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("failed to listen for SIGTERM");
        sigterm.recv().await;
        tracing::info!("Received SIGTERM, initiating graceful shutdown...");
        cancel_sigterm.cancel();
    });

    tokio::spawn(async move {
        let mut sighup = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::hangup())
            .expect("failed to listen for SIGHUP");
        loop {
            sighup.recv().await;
            tracing::info!("Received SIGHUP, reloading configuration...");
            if let Ok(new_addr) = std::env::var("CONTROL_PLANE_HTTP_ADDR") {
                let mut guard = state.control_plane_http_addr.write().await;
                let old_addr = guard.clone();
                *guard = new_addr.clone();
                tracing::info!("Control plane HTTP address: {} -> {}", old_addr, new_addr);
            }
            let new_rate = std::env::var("RATE_LIMIT_RPS")
                .ok()
                .and_then(|v| v.parse::<f64>().ok());
            if let Some(rps) = new_rate {
                tracing::info!("Rate limit RPS (info only, enforced by Go control plane): {}", rps);
            }
            tracing::info!("Configuration reloaded");
        }
    });

    server_handle.await??;

    drop(cert_watcher_handle);

    tracing::info!("Gateway shutdown complete");
    Ok(())
}