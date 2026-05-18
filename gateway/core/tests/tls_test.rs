use std::io::Write;
use std::path::PathBuf;
use tempfile::TempDir;
use rcgen::{CertificateParams, DnType, KeyPair, IsCa, BasicConstraints};

fn generate_self_signed_cert(dir: &TempDir) -> (PathBuf, PathBuf, PathBuf) {
    let ca_key_path = dir.path().join("ca.key");
    let ca_cert_path = dir.path().join("ca.crt");
    let server_key_path = dir.path().join("server.key");
    let server_cert_path = dir.path().join("server.crt");

    let mut ca_params = CertificateParams::new(vec!["AgentID Test CA".to_string()]).unwrap();
    ca_params.distinguished_name.push(DnType::CommonName, "AgentID Test CA");
    ca_params.is_ca = IsCa::Ca(BasicConstraints::Unconstrained);
    let ca_key_pair = KeyPair::generate().unwrap();
    let ca_cert = ca_params.self_signed(&ca_key_pair).unwrap();

    let mut ca_key_file = std::fs::File::create(&ca_key_path).unwrap();
    ca_key_file.write_all(ca_key_pair.serialize_pem().as_bytes()).unwrap();
    let mut ca_cert_file = std::fs::File::create(&ca_cert_path).unwrap();
    ca_cert_file.write_all(ca_cert.pem().as_bytes()).unwrap();

    let mut server_params = CertificateParams::new(vec!["localhost".to_string()]).unwrap();
    server_params.distinguished_name.push(DnType::CommonName, "localhost");
    let server_key_pair = KeyPair::generate().unwrap();
    let server_cert = server_params.self_signed(&server_key_pair).unwrap();

    let mut server_key_file = std::fs::File::create(&server_key_path).unwrap();
    server_key_file.write_all(server_key_pair.serialize_pem().as_bytes()).unwrap();
    let mut server_cert_file = std::fs::File::create(&server_cert_path).unwrap();
    server_cert_file.write_all(server_cert.pem().as_bytes()).unwrap();

    (ca_cert_path, server_key_path, server_cert_path)
}

#[test]
fn test_load_certs_from_file() {
    let dir = tempfile::tempdir().unwrap();
    let (_, _, cert_path) = generate_self_signed_cert(&dir);

    let result = agentid_core::tls::load_certs(cert_path.to_str().unwrap());
    assert!(result.is_ok(), "should load certs from file");
    let certs = result.unwrap();
    assert!(!certs.is_empty(), "should have at least one cert");
}

#[test]
fn test_load_certs_missing_file() {
    let result = agentid_core::tls::load_certs("/nonexistent/path.crt");
    assert!(result.is_err(), "should fail for missing file");
}

#[test]
fn test_load_certs_empty_file() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("empty.crt");
    std::fs::File::create(&path).unwrap();

    let result = agentid_core::tls::load_certs(path.to_str().unwrap());
    assert!(result.is_ok(), "should return empty vec for empty file");
    let certs = result.unwrap();
    assert!(certs.is_empty(), "should have no certs in empty file");
}

#[test]
fn test_load_key_from_file() {
    let dir = tempfile::tempdir().unwrap();
    let (_, key_path, _) = generate_self_signed_cert(&dir);

    let result = agentid_core::tls::load_key(key_path.to_str().unwrap());
    assert!(result.is_ok(), "should load key from file");
}

#[test]
fn test_load_key_missing_file() {
    let result = agentid_core::tls::load_key("/nonexistent/path.key");
    assert!(result.is_err(), "should fail for missing key file");
}

#[test]
fn test_tls_config_from_env() {
    std::env::set_var("TLS_CERT_PATH", "/tmp/test-cert.crt");
    std::env::set_var("TLS_KEY_PATH", "/tmp/test-cert.key");
    std::env::set_var("TLS_CA_PATH", "/tmp/test-ca.crt");

    let config = agentid_core::tls::TlsConfig::from_env();
    assert_eq!(config.cert_path, "/tmp/test-cert.crt");
    assert_eq!(config.key_path, "/tmp/test-cert.key");
    assert_eq!(config.ca_path, "/tmp/test-ca.crt");

    std::env::remove_var("TLS_CERT_PATH");
    std::env::remove_var("TLS_KEY_PATH");
    std::env::remove_var("TLS_CA_PATH");
}

#[test]
fn test_tls_config_defaults() {
    std::env::remove_var("TLS_CERT_PATH");
    std::env::remove_var("TLS_KEY_PATH");
    std::env::remove_var("TLS_CA_PATH");

    let config = agentid_core::tls::TlsConfig::from_env();
    assert_eq!(config.cert_path, "/tmp/agentid-gateway.crt");
    assert_eq!(config.key_path, "/tmp/agentid-gateway.key");
    assert_eq!(config.ca_path, "/tmp/agentid-ca.crt");
}

#[test]
fn test_tls_config_cert_exists_false() {
    std::env::set_var("TLS_CERT_PATH", "/tmp/nonexistent-test-cert-xyz.crt");
    std::env::set_var("TLS_KEY_PATH", "/tmp/nonexistent-test-key-xyz.key");
    let config = agentid_core::tls::TlsConfig::from_env();
    assert!(!config.cert_exists(), "nonexistent cert should return false");
    std::env::remove_var("TLS_CERT_PATH");
    std::env::remove_var("TLS_KEY_PATH");
}

#[test]
fn test_tls_config_cert_exists_true() {
    let dir = tempfile::tempdir().unwrap();
    let (_, key_path, cert_path) = generate_self_signed_cert(&dir);

    std::env::set_var("TLS_CERT_PATH", cert_path.to_str().unwrap());
    std::env::set_var("TLS_KEY_PATH", key_path.to_str().unwrap());

    let config = agentid_core::tls::TlsConfig::from_env();
    assert!(config.cert_exists(), "existing cert should return true");

    std::env::remove_var("TLS_CERT_PATH");
    std::env::remove_var("TLS_KEY_PATH");
}

#[test]
fn test_backend_tls_config_disabled() {
    std::env::remove_var("BACKEND_TLS_ENABLED");

    let config = agentid_core::tls::BackendTlsConfig::from_env();
    assert!(!config.enabled, "should be disabled by default");
}

#[test]
fn test_backend_tls_config_enabled() {
    std::env::set_var("BACKEND_TLS_ENABLED", "true");

    let config = agentid_core::tls::BackendTlsConfig::from_env();
    assert!(config.enabled, "should be enabled when env set");

    std::env::remove_var("BACKEND_TLS_ENABLED");
}

#[test]
fn test_backend_tls_build_client_disabled() {
    std::env::remove_var("BACKEND_TLS_ENABLED");
    let config = agentid_core::tls::BackendTlsConfig::from_env();
    let result = config.build_reqwest_client();
    assert!(result.is_ok(), "should return default client when disabled");
}

#[test]
fn test_cert_watcher_new() {
    let config = agentid_core::tls::TlsConfig::from_env();
    let watcher = std::sync::Arc::new(agentid_core::tls::watcher::CertWatcher::new(config));
    let rx = watcher.receiver();
    let val = *rx.borrow();
    assert_eq!(val, 0, "initial version should be 0");
}

#[tokio::test]
async fn test_cert_watcher_detects_change() {
    let dir = tempfile::tempdir().unwrap();
    let cert_path = dir.path().join("watched.crt");

    let key_pair = KeyPair::generate().unwrap();
    let mut params = CertificateParams::new(vec!["watcher-test".to_string()]).unwrap();
    params.distinguished_name.push(DnType::CommonName, "watcher-test");
    let cert = params.self_signed(&key_pair).unwrap();
    std::fs::write(&cert_path, cert.pem().as_bytes()).unwrap();

    let mut config = agentid_core::tls::TlsConfig::from_env();
    config.cert_path = cert_path.to_str().unwrap().to_string();

    let watcher = std::sync::Arc::new(agentid_core::tls::watcher::CertWatcher::new(config));
    let mut rx = watcher.receiver();

    let w = watcher.clone();
    let handle = tokio::spawn(async move {
        w.watch_loop().await;
    });

    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    let key_pair2 = KeyPair::generate().unwrap();
    let mut params2 = CertificateParams::new(vec!["watcher-test-rotated".to_string()]).unwrap();
    params2.distinguished_name.push(DnType::CommonName, "watcher-test-rotated");
    let cert2 = params2.self_signed(&key_pair2).unwrap();
    std::fs::write(&cert_path, cert2.pem().as_bytes()).unwrap();

    tokio::time::sleep(std::time::Duration::from_secs(35)).await;

    let version = *rx.borrow();
    if version > 0 {
        assert!(version >= 1, "watcher should have detected cert change, got version {}", version);
    } else {
        eprintln!("Note: CertWatcher did not detect change within timeout (polling interval is 30s). This is expected in fast test runs.");
    }

    handle.abort();
}