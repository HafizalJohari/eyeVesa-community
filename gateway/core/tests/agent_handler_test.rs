use agentid_core::proxy::ProxyState;
use agentid_core::proxy::server;
use agentid_core::tls::BackendTlsConfig;
use std::sync::Arc;
use tokio::sync::Mutex;

use wiremock::{MockServer};

fn make_state(mock_server: &MockServer) -> Arc<ProxyState> {
    Arc::new(ProxyState {
        control_plane: Arc::new(Mutex::new(None)),
        control_plane_addr: "http://localhost:9999".to_string(),
        control_plane_http_addr: mock_server.uri().replace("http://", "").replace("https://", ""),
        http_client: reqwest::Client::builder()
            .no_proxy()
            .build()
            .expect("build client"),
        backend_tls: BackendTlsConfig {
            enabled: false,
            ca_path: String::new(),
            cert_path: String::new(),
            key_path: String::new(),
            server_name: String::new(),
        },
    })
}

async fn start_proxy(state: Arc<ProxyState>) -> std::net::SocketAddr {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move {
        loop {
            let (stream, remote_addr) = listener.accept().await.expect("accept");
            let state = state.clone();
            tokio::spawn(async move {
                use hyper::service::service_fn;
                use hyper_util::rt::TokioIo;
                let service = service_fn(move |req| server::handle_request(req, state.clone()));
                if let Err(e) = hyper::server::conn::http1::Builder::new()
                    .serve_connection(TokioIo::new(stream), service)
                    .await
                {
                    eprintln!("Error serving connection from {}: {}", remote_addr, e);
                }
            });
        }
    });
    addr
}

#[tokio::test]
async fn test_register_bad_json() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/register", proxy_addr.port()))
        .body("not json")
        .header("content-type", "application/json")
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 400);
}

#[tokio::test]
async fn test_register_missing_fields() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/register", proxy_addr.port()))
        .json(&serde_json::json!({}))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 400);
}

#[tokio::test]
async fn test_register_grpc_unavailable() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/register", proxy_addr.port()))
        .json(&serde_json::json!({
            "name": "test-agent",
            "owner": "test-owner"
        }))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 502);
    let body: serde_json::Value = resp.json().await.expect("json");
    assert!(body["error"].as_str().unwrap_or("").contains("control plane"));
}

#[tokio::test]
async fn test_authorize_bad_json() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/auth", proxy_addr.port()))
        .body("bad")
        .header("content-type", "application/json")
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 400);
}

#[tokio::test]
async fn test_authorize_grpc_unavailable() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/auth", proxy_addr.port()))
        .json(&serde_json::json!({
            "agent_id": "agent-1",
            "action": "read"
        }))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 502);
}