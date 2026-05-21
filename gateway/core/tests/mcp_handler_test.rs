use agentid_core::proxy::ProxyState;
use agentid_core::proxy::server;
use agentid_core::tls::BackendTlsConfig;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::{Mutex, RwLock};

use wiremock::MockServer;

fn make_state(mock_server: &MockServer) -> Arc<ProxyState> {
    Arc::new(ProxyState {
        control_plane: Arc::new(Mutex::new(None)),
        control_plane_addr: "http://localhost:9999".to_string(),
        control_plane_http_addr: Arc::new(tokio::sync::RwLock::new(mock_server.uri().replace("http://", "").replace("https://", ""))),
        central_airport_url: None,
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

async fn start_proxy(state: Arc<ProxyState>) -> SocketAddr {
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
async fn test_health_endpoint() {
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
    let body = resp.text().await.expect("body");
    assert_eq!(body, "ok");
}

#[tokio::test]
async fn test_404_unknown_path() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/unknown", proxy_addr.port()))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 404);
}

#[tokio::test]
async fn test_mcp_initialize() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {}
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(resp_body["jsonrpc"], "2.0");
    assert_eq!(resp_body["id"], 1);
    assert!(resp_body["result"]["protocolVersion"].is_string());
    assert_eq!(resp_body["result"]["serverInfo"]["name"], "agentid-gateway");
}

#[tokio::test]
async fn test_mcp_resources_list() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 2,
        "method": "resources/list"
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert!(resp_body["result"]["resources"].is_array());
}

#[tokio::test]
async fn test_mcp_prompts_list() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 3,
        "method": "prompts/list"
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert!(resp_body["result"]["prompts"].is_array());
}

#[tokio::test]
async fn test_mcp_parse_error() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .body("not valid json {{{")
        .header("content-type", "application/json")
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(resp_body["error"]["code"], -32700);
}

#[tokio::test]
async fn test_mcp_method_not_found() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 4,
        "method": "nonexistent/method"
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(resp_body["error"]["code"], -32601);
}

#[tokio::test]
async fn test_mcp_tools_call_missing_args() {
    let mock_server = MockServer::start().await;
    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "jsonrpc": "2.0",
        "id": 5,
        "method": "tools/call",
        "params": {}
    });
    let resp = client
        .post(format!("http://127.0.0.1:{}/v1/mcp", proxy_addr.port()))
        .json(&body)
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let resp_body: serde_json::Value = resp.json().await.expect("json");
    assert!(resp_body["result"]["isError"].as_bool().unwrap_or(false));
}