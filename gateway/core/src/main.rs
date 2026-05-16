mod crypto;
mod proxy;

use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env().add_directive("info".parse()?))
        .init();

    tracing::info!("AgentID Core Gateway starting...");

    let addr = std::net::SocketAddr::from(([0, 0, 0, 0], 9443));
    tracing::info!("Listening on {}", addr);

    proxy::server::run(addr).await?;

    Ok(())
}