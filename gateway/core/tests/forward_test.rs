use agentid_core::proxy::forward::control_plane_http_base;
use agentid_core::proxy::server;
use agentid_core::proxy::ProxyState;
use agentid_core::tls::BackendTlsConfig;
use std::sync::Arc;
use tokio::sync::Mutex;

use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn make_state(mock_server: &MockServer) -> Arc<ProxyState> {
    Arc::new(ProxyState {
        control_plane: Arc::new(Mutex::new(None)),
        control_plane_addr: "http://localhost:9999".to_string(),
        control_plane_http_addr: Arc::new(tokio::sync::RwLock::new(
            mock_server
                .uri()
                .replace("http://", "")
                .replace("https://", ""),
        )),
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

#[test]
fn test_control_plane_http_base_accepts_full_https_url() {
    let base = control_plane_http_base("https://gateway-control.example.run.app/", false);
    assert_eq!(base, "https://gateway-control.example.run.app");
}

#[test]
fn test_control_plane_http_base_adds_scheme_for_host() {
    assert_eq!(
        control_plane_http_base("gateway-control.example.run.app", false),
        "http://gateway-control.example.run.app"
    );
    assert_eq!(
        control_plane_http_base("gateway-control.example.run.app", true),
        "https://gateway-control.example.run.app"
    );
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
async fn test_forward_get_audit() {
    let mock_server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/audit"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "entries": []
        })))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/v1/audit", proxy_addr.port()))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let body: serde_json::Value = resp.json().await.expect("json");
    assert!(body["entries"].is_array());
}

#[tokio::test]
async fn test_forward_post_ptv_attest() {
    let mock_server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/v1/ptv/attest"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "status": "ok"
        })))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!(
            "http://127.0.0.1:{}/v1/ptv/attest",
            proxy_addr.port()
        ))
        .json(&serde_json::json!({"agent_id": "test"}))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(body["status"], "ok");
}

#[tokio::test]
async fn test_forward_post_hitl_approve() {
    let mock_server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/v1/hitl/approve"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "approved": true
        })))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!(
            "http://127.0.0.1:{}/v1/hitl/approve",
            proxy_addr.port()
        ))
        .json(&serde_json::json!({"approval_id": "abc"}))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(body["approved"], true);
}

#[tokio::test]
async fn test_forward_get_trust_score() {
    let mock_server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/agents/agent-123/trust"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "trust_score": 0.75
        })))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!(
            "http://127.0.0.1:{}/v1/agents/agent-123/trust",
            proxy_addr.port()
        ))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
    let body: serde_json::Value = resp.json().await.expect("json");
    assert_eq!(body["trust_score"], 0.75);
}

#[tokio::test]
async fn test_forward_catchall_v1() {
    let mock_server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/v1/some/new/endpoint"))
        .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
            "ok": true
        })))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .post(format!(
            "http://127.0.0.1:{}/v1/some/new/endpoint",
            proxy_addr.port()
        ))
        .json(&serde_json::json!({}))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 200);
}

#[tokio::test]
async fn test_forward_backend_502() {
    let mock_server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/audit"))
        .respond_with(ResponseTemplate::new(502))
        .mount(&mock_server)
        .await;

    let state = make_state(&mock_server);
    let proxy_addr = start_proxy(state).await;

    let client = reqwest::Client::new();
    let resp = client
        .get(format!("http://127.0.0.1:{}/v1/audit", proxy_addr.port()))
        .send()
        .await
        .expect("request");

    assert_eq!(resp.status(), 502);
}
