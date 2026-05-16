use http_body_util::BodyExt;
use hyper::body::Incoming;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Request, Response};
use std::net::SocketAddr;
use tokio::net::TcpListener;

use crate::proxy::mcp_handler;

pub async fn run(addr: SocketAddr) -> Result<(), Box<dyn std::error::Error>> {
    let listener = TcpListener::bind(addr).await?;
    tracing::info!("Proxy server bound to {}", addr);

    loop {
        let (stream, remote_addr) = listener.accept().await?;
        tracing::debug!("Accepted connection from {}", remote_addr);

        tokio::spawn(async move {
            let service = service_fn(handle_request);
            if let Err(e) = http1::Builder::new()
                .serve_connection(hyper_util::rt::TokioIo::new(stream), service)
                .await
            {
                tracing::error!("Error serving connection from {}: {}", remote_addr, e);
            }
        });
    }
}

async fn handle_request(
    req: Request<Incoming>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path();
    let method = req.method().clone();

    tracing::info!("{} {}", method, path);

    match (method.as_str(), path) {
        ("POST", "/v1/mcp") => mcp_handler::handle_mcp_request(req).await,
        ("GET", "/health") => Ok(Response::builder()
            .status(200)
            .body("ok".to_string())?),
        _ => Ok(Response::builder()
            .status(404)
            .body("not found".to_string())?),
    }
}