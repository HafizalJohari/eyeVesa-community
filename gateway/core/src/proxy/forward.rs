use crate::proxy::ProxyState;
use hyper::body::Incoming;
use hyper::{Request, Response};
use std::sync::Arc;

pub fn control_plane_http_base(control_plane_http_addr: &str, backend_tls_enabled: bool) -> String {
    let addr = control_plane_http_addr.trim_end_matches('/');
    if addr.starts_with("http://") || addr.starts_with("https://") {
        return addr.to_string();
    }

    let scheme = if backend_tls_enabled { "https" } else { "http" };
    format!("{}://{}", scheme, addr)
}

pub async fn forward_to_control_plane(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path().to_string();
    let method = req.method().clone();
    let (parts, body) = req.into_parts();
    let bytes = crate::proxy::collect_body(body).await?;

    let client = &state.http_client;
    let cp_addr = state.control_plane_http_addr.read().await.clone();
    let url = format!(
        "{}{}",
        control_plane_http_base(&cp_addr, state.backend_tls.enabled),
        path
    );

    let mut builder = match method.as_str() {
        "GET" => client.get(&url),
        "POST" => client.post(&url),
        "PUT" => client.put(&url),
        "DELETE" => client.delete(&url),
        "PATCH" => client.patch(&url),
        _ => client.get(&url),
    };

    if let Some(ct) = parts.headers.get("content-type") {
        builder = builder.header("content-type", ct);
    }

    if let Some(auth) = parts.headers.get("authorization") {
        builder = builder.header("authorization", auth);
    }

    if let Some(api_key) = parts.headers.get("x-api-key") {
        builder = builder.header("x-api-key", api_key);
    }

    if !bytes.is_empty() {
        builder = builder.body(bytes);
    }

    let resp = builder
        .send()
        .await
        .map_err(|e| format!("forward error: {}", e))?;
    let status = resp.status();
    let resp_ct = resp.headers().get("content-type").cloned();
    let body_text = resp
        .text()
        .await
        .map_err(|e| format!("forward body error: {}", e))?;

    let mut response_builder = Response::builder().status(status.as_u16());
    if let Some(ct) = resp_ct {
        response_builder = response_builder.header("content-type", ct);
    } else if !body_text.is_empty() {
        response_builder = response_builder.header("content-type", "application/json");
    }
    Ok(response_builder.body(body_text)?)
}

pub async fn forward_to_central_airport(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path().to_string();
    let method = req.method().clone();
    let (parts, body) = req.into_parts();
    let bytes = crate::proxy::collect_body(body).await?;

    let central_url = match &state.central_airport_url {
        Some(url) => url.clone(),
        None => {
            return Ok(Response::builder()
                .status(503)
                .header("content-type", "application/json")
                .body(r#"{"error":"CENTRAL_AIRPORT_URL not configured"}"#.to_string())?);
        }
    };

    let url = format!(
        "{}/v1/federation/{}",
        central_url.trim_end_matches('/'),
        path.strip_prefix("/v1/federation/international/")
            .unwrap_or(&path)
    );
    let client = &state.http_client;

    let mut builder = match method.as_str() {
        "GET" => client.get(&url),
        "POST" => client.post(&url),
        "PUT" => client.put(&url),
        "DELETE" => client.delete(&url),
        "PATCH" => client.patch(&url),
        _ => client.get(&url),
    };

    if let Some(ct) = parts.headers.get("content-type") {
        builder = builder.header("content-type", ct);
    }

    if let Some(auth) = parts.headers.get("authorization") {
        builder = builder.header("authorization", auth);
    }

    if let Some(api_key) = parts.headers.get("x-api-key") {
        builder = builder.header("x-api-key", api_key);
    }

    if !bytes.is_empty() {
        builder = builder.body(bytes);
    }

    let resp = builder
        .send()
        .await
        .map_err(|e| format!("central airport forward error: {}", e))?;
    let status = resp.status();
    let resp_ct = resp.headers().get("content-type").cloned();
    let body_text = resp
        .text()
        .await
        .map_err(|e| format!("central airport body error: {}", e))?;

    let mut response_builder = Response::builder().status(status.as_u16());
    if let Some(ct) = resp_ct {
        response_builder = response_builder.header("content-type", ct);
    } else if !body_text.is_empty() {
        response_builder = response_builder.header("content-type", "application/json");
    }
    Ok(response_builder.body(body_text)?)
}
