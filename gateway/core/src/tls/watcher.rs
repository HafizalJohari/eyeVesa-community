use crate::tls::TlsConfig;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use tokio::sync::watch;

pub struct CertWatcher {
    config: TlsConfig,
    version: AtomicU64,
    tx: watch::Sender<u64>,
    rx: watch::Receiver<u64>,
}

impl CertWatcher {
    pub fn new(config: TlsConfig) -> Self {
        let (tx, rx) = watch::channel(0);
        Self {
            config,
            version: AtomicU64::new(0),
            tx,
            rx,
        }
    }

    pub fn receiver(&self) -> watch::Receiver<u64> {
        self.rx.clone()
    }

    pub async fn watch_loop(self: Arc<Self>) {
        let mut last_mtime: Option<std::time::SystemTime> = None;

        loop {
            tokio::time::sleep(std::time::Duration::from_secs(30)).await;

            let cert_path = &self.config.cert_path;
            let metadata = match std::fs::metadata(cert_path) {
                Ok(m) => m,
                Err(_) => continue,
            };

            let modified = metadata.modified().ok();

            if last_mtime != modified {
                match crate::tls::load_certs(cert_path) {
                    Ok(certs) if !certs.is_empty() => {
                        tracing::info!(
                            "Certificate file changed, reloading (version={})",
                            self.version.load(Ordering::SeqCst) + 1
                        );
                        last_mtime = modified;
                        let v = self.version.fetch_add(1, Ordering::SeqCst) + 1;
                        let _ = self.tx.send(v);
                    }
                    Ok(_) => {
                        tracing::warn!("Certificate file changed but no certs found");
                    }
                    Err(e) => {
                        tracing::warn!("Failed to load changed certificate: {}", e);
                    }
                }
            }
        }
    }
}