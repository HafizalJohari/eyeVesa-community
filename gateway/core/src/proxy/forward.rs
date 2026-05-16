use crate::proxy::ProxyState;
use hyper::body::Incoming;
use hyper::{Request, Response};
use std::sync::Arc;

pub async fn forward_to_control_plane(
    req: Request<Incoming>,
    state: Arc<ProxyState>,
) -> Result<Response<String>, Box<dyn std::error::Error + Send + Sync>> {
    let path = req.uri().path().to_string();
    let method = req.method().clone();
    let (parts, body) = req.into_parts();
    let bytes = crate::proxy::collect_body(body).await?;

    let client = &state.http_client;
    let url = format!("http://{}{}", state.control_plane_http_addr, path);

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

    if !bytes.is_empty() {
        builder = builder.body(bytes);
    }

    let resp = builder.send().await.map_err(|e| format!("forward error: {}", e))?;
    let status = resp.status();
    let body_text = resp.text().await.map_err(|e| format!("forward body error: {}", e))?;

    Ok(Response::builder()
        .status(status.as_u16())
        .header("content-type", "application/json")
        .body(body_text)?)
}