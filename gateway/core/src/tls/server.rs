use crate::tls::TlsConfig;
use std::sync::Arc;
use tokio::net::TcpListener;
use tokio_rustls::TlsAcceptor;

pub async fn run_tls(
    addr: std::net::SocketAddr,
    tls_config: &TlsConfig,
    state: std::sync::Arc<crate::proxy::ProxyState>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let certs = crate::tls::load_certs(&tls_config.cert_path)?;
    let key = crate::tls::load_key(&tls_config.key_path)?;

    let server_config = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(certs, key)?;

    let acceptor = TlsAcceptor::from(Arc::new(server_config));
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("TLS proxy server bound to {}", addr);

    loop {
        let (stream, remote_addr) = listener.accept().await?;
        let acceptor = acceptor.clone();
        let state = state.clone();

        tokio::spawn(async move {
            match acceptor.accept(stream).await {
                Ok(tls_stream) => {
                    tracing::info!("TLS connection from {}", remote_addr);
                    let service = hyper::service::service_fn(move |req| {
                        crate::proxy::server::handle_request(req, state.clone())
                    });
                    if let Err(e) = hyper::server::conn::http1::Builder::new()
                        .serve_connection(hyper_util::rt::TokioIo::new(tls_stream), service)
                        .await
                    {
                        tracing::error!("Error serving TLS connection from {}: {}", remote_addr, e);
                    }
                }
                Err(e) => {
                    tracing::error!("TLS handshake failed from {}: {}", remote_addr, e);
                }
            }
        });
    }
}

pub async fn run_mtls(
    addr: std::net::SocketAddr,
    tls_config: &TlsConfig,
    state: std::sync::Arc<crate::proxy::ProxyState>,
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
        tracing::warn!("CA certificate not found at {}, using permissive mTLS (accepts any client cert)", tls_config.ca_path);
    }

    let client_verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(root_store))
        .allow_unauthenticated()
        .build()?;

    let server_config = rustls::ServerConfig::builder()
        .with_client_cert_verifier(client_verifier)
        .with_single_cert(certs, key)?;

    let acceptor = TlsAcceptor::from(Arc::new(server_config));
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("mTLS proxy server bound to {}", addr);

    loop {
        let (stream, remote_addr) = listener.accept().await?;
        let acceptor = acceptor.clone();
        let state = state.clone();

        tokio::spawn(async move {
            match acceptor.accept(stream).await {
                Ok(tls_stream) => {
                    tracing::info!("mTLS connection from {}", remote_addr);
                    let service = hyper::service::service_fn(move |req| {
                        crate::proxy::server::handle_request(req, state.clone())
                    });
                    if let Err(e) = hyper::server::conn::http1::Builder::new()
                        .serve_connection(hyper_util::rt::TokioIo::new(tls_stream), service)
                        .await
                    {
                        tracing::error!("Error serving mTLS connection from {}: {}", remote_addr, e);
                    }
                }
                Err(e) => {
                    tracing::error!("mTLS handshake failed from {}: {}", remote_addr, e);
                }
            }
        });
    }
}