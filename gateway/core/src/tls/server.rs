use crate::tls::TlsConfig;
use crate::proxy::server::DRAINING;
use std::sync::Arc;
use std::sync::atomic::Ordering;
use tokio::net::TcpListener;
use tokio_rustls::TlsAcceptor;
use tokio_util::sync::CancellationToken;

pub async fn run_tls(
    addr: std::net::SocketAddr,
    tls_config: &TlsConfig,
    state: std::sync::Arc<crate::proxy::ProxyState>,
    cancel: CancellationToken,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let certs = crate::tls::load_certs(&tls_config.cert_path)?;
    let key = crate::tls::load_key(&tls_config.key_path)?;

    let server_config = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(certs, key)?;

    let acceptor = TlsAcceptor::from(Arc::new(server_config));
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("TLS proxy server bound to {}", addr);

    let mut conn_handles: Vec<tokio::task::JoinHandle<()>> = Vec::new();

    loop {
        tokio::select! {
            accept_result = listener.accept() => {
                if DRAINING.load(Ordering::SeqCst) {
                    if let Ok((stream, _)) = accept_result {
                        drop(stream);
                    }
                    continue;
                }

                let (stream, remote_addr) = accept_result?;
                let acceptor = acceptor.clone();
                let state = state.clone();
                let cancel_clone = cancel.clone();

                let handle = tokio::spawn(async move {
                    match acceptor.accept(stream).await {
                        Ok(tls_stream) => {
                            tracing::info!("TLS connection from {}", remote_addr);
                            let service = hyper::service::service_fn(move |req| {
                                crate::proxy::server::handle_request(req, state.clone())
                            });
                            let conn = hyper::server::conn::http1::Builder::new()
                                .serve_connection(hyper_util::rt::TokioIo::new(tls_stream), service);

                            tokio::select! {
                                result = conn => {
                                    if let Err(e) = result {
                                        tracing::error!("Error serving TLS connection from {}: {}", remote_addr, e);
                                    }
                                }
                                _ = cancel_clone.cancelled() => {
                                    tracing::debug!("TLS connection cancelled for {}", remote_addr);
                                }
                            }
                        }
                        Err(e) => {
                            tracing::error!("TLS handshake failed from {}: {}", remote_addr, e);
                        }
                    }
                });
                conn_handles.push(handle);
            }
            _ = cancel.cancelled() => {
                tracing::info!("Shutdown signal received (TLS), draining connections...");
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
                        tracing::warn!("Drain timeout reached, {} TLS connections still active", conn_handles.len());
                        break;
                    }
                    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                }

                tracing::info!("All TLS connections drained, shutting down");
                return Ok(());
            }
        }
    }
}

pub async fn run_mtls(
    addr: std::net::SocketAddr,
    tls_config: &TlsConfig,
    state: std::sync::Arc<crate::proxy::ProxyState>,
    cancel: CancellationToken,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let certs = crate::tls::load_certs(&tls_config.cert_path)?;
    let key = crate::tls::load_key(&tls_config.key_path)?;

    let mut root_store = rustls::RootCertStore::empty();
    if std::path::Path::new(&tls_config.ca_path).exists() {
        let ca_certs = crate::tls::load_certs(&tls_config.ca_path)?;
        for cert in ca_certs {
            if let Err(e) = root_store.add(cert) {
                tracing::warn!("Failed to add CA cert: {}", e);
            }
        }
        tracing::info!("Loaded CA certificate from {}", tls_config.ca_path);
    } else {
        return Err(format!("CA certificate not found at {}. mTLS requires valid client certificates. Set TLS_CA_PATH environment variable.", tls_config.ca_path).into());
    }

    let client_verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(root_store))
        .build()?;

    let server_config = rustls::ServerConfig::builder()
        .with_client_cert_verifier(client_verifier)
        .with_single_cert(certs, key)?;

    let acceptor = TlsAcceptor::from(Arc::new(server_config));
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("mTLS proxy server bound to {}", addr);

    let mut conn_handles: Vec<tokio::task::JoinHandle<()>> = Vec::new();

    loop {
        tokio::select! {
            accept_result = listener.accept() => {
                if DRAINING.load(Ordering::SeqCst) {
                    if let Ok((stream, _)) = accept_result {
                        drop(stream);
                    }
                    continue;
                }

                let (stream, remote_addr) = accept_result?;
                let acceptor = acceptor.clone();
                let state = state.clone();
                let cancel_clone = cancel.clone();

                let handle = tokio::spawn(async move {
                    match acceptor.accept(stream).await {
                        Ok(tls_stream) => {
                            tracing::info!("mTLS connection from {}", remote_addr);
                            let service = hyper::service::service_fn(move |req| {
                                crate::proxy::server::handle_request(req, state.clone())
                            });
                            let conn = hyper::server::conn::http1::Builder::new()
                                .serve_connection(hyper_util::rt::TokioIo::new(tls_stream), service);

                            tokio::select! {
                                result = conn => {
                                    if let Err(e) = result {
                                        tracing::error!("Error serving mTLS connection from {}: {}", remote_addr, e);
                                    }
                                }
                                _ = cancel_clone.cancelled() => {
                                    tracing::debug!("mTLS connection cancelled for {}", remote_addr);
                                }
                            }
                        }
                        Err(e) => {
                            tracing::error!("mTLS handshake failed from {}: {}", remote_addr, e);
                        }
                    }
                });
                conn_handles.push(handle);
            }
            _ = cancel.cancelled() => {
                tracing::info!("Shutdown signal received (mTLS), draining connections...");
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
                        tracing::warn!("Drain timeout reached, {} mTLS connections still active", conn_handles.len());
                        break;
                    }
                    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                }

                tracing::info!("All mTLS connections drained, shutting down");
                return Ok(());
            }
        }
    }
}