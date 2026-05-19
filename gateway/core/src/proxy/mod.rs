pub mod mcp_handler;
pub mod agent_handler;
pub mod forward;
pub mod server;

use std::sync::Arc;
use tokio::sync::{Mutex, RwLock};

use crate::grpc::ControlPlaneClient;
use crate::tls::BackendTlsConfig;

pub struct ProxyState {
    pub control_plane: Arc<Mutex<Option<ControlPlaneClient>>>,
    pub control_plane_addr: String,
    pub control_plane_http_addr: Arc<RwLock<String>>,
    pub central_airport_url: Option<String>,
    pub http_client: reqwest::Client,
    pub backend_tls: BackendTlsConfig,
}

pub async fn collect_body(body: hyper::body::Incoming) -> Result<Vec<u8>, Box<dyn std::error::Error + Send + Sync>> {
    use http_body_util::BodyExt;
    let bytes = body.collect().await?.to_bytes();
    Ok(bytes.to_vec())
}