pub mod server;
pub mod watcher;

use std::fs;
use std::path::Path;

pub struct TlsConfig {
    pub cert_path: String,
    pub key_path: String,
    pub ca_path: String,
    #[allow(dead_code)]
    pub require_client_cert: bool,
}

#[allow(dead_code)]
pub struct BackendTlsConfig {
    pub enabled: bool,
    pub ca_path: String,
    pub cert_path: String,
    pub key_path: String,
    pub server_name: String,
}

impl TlsConfig {
    pub fn from_env() -> Self {
        Self {
            cert_path: std::env::var("TLS_CERT_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-gateway.crt".to_string()),
            key_path: std::env::var("TLS_KEY_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-gateway.key".to_string()),
            ca_path: std::env::var("TLS_CA_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-ca.crt".to_string()),
            require_client_cert: std::env::var("TLS_REQUIRE_CLIENT_CERT")
                .unwrap_or_else(|_| "false".to_string())
                == "true",
        }
    }

    #[allow(dead_code)]
    pub fn cert_exists(&self) -> bool {
        Path::new(&self.cert_path).exists() && Path::new(&self.key_path).exists()
    }
}

impl BackendTlsConfig {
    pub fn from_env() -> Self {
        let enabled = std::env::var("BACKEND_TLS_ENABLED")
            .unwrap_or_else(|_| "false".to_string())
            == "true";
        Self {
            enabled,
            ca_path: std::env::var("BACKEND_TLS_CA_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-ca.crt".to_string()),
            cert_path: std::env::var("BACKEND_TLS_CERT_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-gateway.crt".to_string()),
            key_path: std::env::var("BACKEND_TLS_KEY_PATH")
                .unwrap_or_else(|_| "/tmp/agentid-gateway.key".to_string()),
            server_name: std::env::var("BACKEND_TLS_SERVER_NAME")
                .unwrap_or_else(|_| "gateway-control".to_string()),
        }
    }

    pub fn build_reqwest_client(&self) -> Result<reqwest::Client, Box<dyn std::error::Error + Send + Sync>> {
        if !self.enabled {
            return Ok(reqwest::Client::new());
        }

        let root_cert = load_certs(&self.ca_path)?;
        let client_certs = load_certs(&self.cert_path)?;
        let client_key = load_key(&self.key_path)?;

        let root_store = {
            let mut store = rustls::RootCertStore::empty();
            for cert in root_cert {
                store.add(cert)?;
            }
            store
        };

        let config = rustls::ClientConfig::builder()
            .with_root_certificates(root_store)
            .with_client_auth_cert(client_certs, client_key)?;

        let tls_client = reqwest::Client::builder()
            .use_preconfigured_tls(config)
            .build()?;

        Ok(tls_client)
    }
}

pub fn load_certs(path: &str) -> Result<Vec<rustls::pki_types::CertificateDer<'static>>, Box<dyn std::error::Error + Send + Sync>> {
    let data = fs::read(path)?;
    let certs: Vec<_> = rustls_pemfile::certs(&mut &data[..])
        .collect::<Result<Vec<_>, _>>()?;
    Ok(certs)
}

pub fn load_key(path: &str) -> Result<rustls::pki_types::PrivateKeyDer<'static>, Box<dyn std::error::Error + Send + Sync>> {
    let data = fs::read(path)?;
    
    let pkcs8_keys: Vec<_> = rustls_pemfile::pkcs8_private_keys(&mut &data[..])
        .collect::<Result<Vec<_>, _>>()?;
    
    if !pkcs8_keys.is_empty() {
        return Ok(rustls::pki_types::PrivateKeyDer::Pkcs8(pkcs8_keys.into_iter().next().unwrap()));
    }
    
    let rsa_keys: Vec<_> = rustls_pemfile::rsa_private_keys(&mut &data[..])
        .collect::<Result<Vec<_>, _>>()?;
    if !rsa_keys.is_empty() {
        return Ok(rustls::pki_types::PrivateKeyDer::Pkcs1(rsa_keys.into_iter().next().unwrap()));
    }
    
    let ec_keys: Vec<_> = rustls_pemfile::ec_private_keys(&mut &data[..])
        .collect::<Result<Vec<_>, _>>()?;
    if !ec_keys.is_empty() {
        return Ok(rustls::pki_types::PrivateKeyDer::Sec1(ec_keys.into_iter().next().unwrap()));
    }
    
    Err("No private key found".into())
}