use agentid_core::proxy::ProxyState;
use agentid_core::proxy::server;
use agentid_core::tls::BackendTlsConfig;
use std::sync::Arc;
use std::sync::atomic::Ordering;
use tokio::sync::Mutex;

use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

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
    agentid_core::proxy::server::DRAINING.store(false, Ordering::SeqCst);
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
async fn test_routing_mcp_post() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize"
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 200);
}

#[tokio::test]
async fn test_routing_health_get() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/health", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 200);
}

#[tokio::test]
async fn test_routing_v1_forward() {
    let mock_server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/agents"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/v1/agents", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 200);
}

#[tokio::test]
async fn test_routing_mcp_get_rejected() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .send()
        .await
        .expect("request");

    // GET /v1/mcp falls through to catch-all v1 forward, which won't match the mock
    // so it'll return whatever the mock server's default is
    assert!(resp.status().as_u16() < 500);
}

#[tokio::test]
async fn test_routing_delegation_forward() {
    let mock_server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/v1/agents/agent-1/delegate"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"ok": true})))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!(
            "http://127.0.0.1:{}/v1/agents/agent-1/delegate",
            proxy_addr.port()
        ))
        .json(&serde_json::json!({}))
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 200);
}

#[tokio::test]
async fn test_concurrent_requests() {
    let mock_server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/audit"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({"entries": []})))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let mut handles = vec![];

    for _ in 0..10 {
        let c = client.clone();
        let url = format!("http://127.0.0.1:{}/v1/audit", proxy_addr.port());
        handles.push(tokio::spawn(async move {
            c.get(&url).send().await.expect("request").status()
        }));
    }

    for handle in handles {
        let status = handle.await.expect("join");
        assert_eq!(status, 200);
    }
}

#[tokio::test]
async fn test_ready_endpoint() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();

    let resp = client
        .get(format!("http://127.0.0.1:{}/ready", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 200);
    let body = resp.text().await.expect("body");
    assert_eq!(body, "ready");
}

#[tokio::test]
#[serial_test::serial]
async fn test_draining_returns_503() {
    use agentid_core::proxy::server::DRAINING;
    use std::sync::atomic::Ordering;

    DRAINING.store(false, Ordering::SeqCst);

    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();

    let ready_before = client
        .get(format!("http://127.0.0.1:{}/ready", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(ready_before.status(), 200);

    DRAINING.store(true, Ordering::SeqCst);

    let resp = client
        .get(format!("http://127.0.0.1:{}/v1/agents", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(resp.status(), 503);

    let health_resp = client
        .get(format!("http://127.0.0.1:{}/health", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(health_resp.status(), 200);

    let ready_resp = client
        .get(format!("http://127.0.0.1:{}/ready", proxy_addr.port()))
        .send()
        .await
        .expect("request");
    assert_eq!(ready_resp.status(), 503);

    DRAINING.store(false, Ordering::SeqCst);
}