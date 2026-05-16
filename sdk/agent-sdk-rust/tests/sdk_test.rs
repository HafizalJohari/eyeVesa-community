use agentid_sdk::client::AgentClient;
use agentid_sdk::AgentConfig;
use ed25519_dalek::SigningKey;
use rand::rngs::OsRng;

#[test]
fn test_agent_client_new() {
    let config = AgentConfig {
        agent_id: uuid::Uuid::new_v4(),
        name: "test-agent".to_string(),
        owner: "test-team".to_string(),
        gateway_endpoint: "http://localhost:9443".to_string(),
    };

    let signing_key = SigningKey::generate(&mut OsRng);
    let client = AgentClient::new(config, signing_key);

    assert_eq!(client.name(), "test-agent");
    assert_eq!(client.owner(), "test-team");
    assert_eq!(client.gateway_endpoint(), "http://localhost:9443");
    assert!(!client.is_registered());
    assert_eq!(client.trust_score(), 1.0);
}

#[test]
fn test_agent_client_update_trust() {
    let config = AgentConfig {
        agent_id: uuid::Uuid::new_v4(),
        name: "test-agent".to_string(),
        owner: "test-team".to_string(),
        gateway_endpoint: "http://localhost:9443".to_string(),
    };

    let signing_key = SigningKey::generate(&mut OsRng);
    let mut client = AgentClient::new(config, signing_key);

    assert_eq!(client.trust_score(), 1.0);
    client.update_trust_score(0.75);
    assert_eq!(client.trust_score(), 0.75);
}

#[test]
fn test_agent_config_serialization() {
    let id = uuid::Uuid::new_v4();
    let config = AgentConfig {
        agent_id: id,
        name: "test-agent".to_string(),
        owner: "test-team".to_string(),
        gateway_endpoint: "http://localhost:9443".to_string(),
    };

    let json = serde_json::to_string(&config).unwrap();
    let deserialized: AgentConfig = serde_json::from_str(&json).unwrap();

    assert_eq!(deserialized.agent_id, id);
    assert_eq!(deserialized.name, "test-agent");
    assert_eq!(deserialized.owner, "test-team");
    assert_eq!(deserialized.gateway_endpoint, "http://localhost:9443");
}

#[test]
fn test_authorize_result_deserialization() {
    let json = r#"{"allowed":true,"requires_hitl":false,"reason":"ok","trust_delta":0.1}"#;
    let result: agentid_sdk::AuthorizeResult = serde_json::from_str(json).unwrap();

    assert!(result.allowed);
    assert!(!result.requires_hitl);
    assert_eq!(result.reason, "ok");
    assert!((result.trust_delta - 0.1).abs() < f64::EPSILON);
}

#[test]
fn test_hitl_approval_deserialization() {
    let json = r#"{"approval_id":"550e8400-e29b-41d4-a716-446655440000","agent_id":"agent-1","action":"bank_transfer","status":"pending","expires_at":"2026-01-01T00:00:00Z"}"#;
    let result: agentid_sdk::HitlApproval = serde_json::from_str(json).unwrap();

    assert_eq!(result.status, "pending");
    assert_eq!(result.action, "bank_transfer");
}

#[test]
fn test_invoke_result_deserialization() {
    let json = r#"{"success":true,"data":{"result":"ok"},"trust_score":0.95}"#;
    let result: agentid_sdk::InvokeResult = serde_json::from_str(json).unwrap();

    assert!(result.success);
    assert!((result.trust_score - 0.95).abs() < f64::EPSILON);
}

#[test]
fn test_delegate_result_deserialization() {
    let json = r#"{"delegation_id":"550e8400-e29b-41d4-a716-446655440000","status":"active"}"#;
    let result: agentid_sdk::DelegateResult = serde_json::from_str(json).unwrap();

    assert_eq!(result.status, "active");
}

#[test]
fn test_ptv_attest_result_deserialization() {
    let json = r#"{"attestation":{"agent_id":"agent-1"},"tpm_signature":"c2ln","quote":"cXVvdGU="}"#;
    let result: agentid_sdk::PtvAttestResult = serde_json::from_str(json).unwrap();

    assert_eq!(result.tpm_signature, "c2ln");
    assert_eq!(result.quote, "cXVvdGU=");
}

#[test]
fn test_ptv_bind_result_deserialization() {
    let json = r#"{"binding_id":"550e8400-e29b-41d4-a716-446655440000","agent_id":"agent-1","platform":"linux-tpm2","transformed_at":1700000000,"expires_at":1700086400}"#;
    let result: agentid_sdk::PtvBindResult = serde_json::from_str(json).unwrap();

    assert_eq!(result.agent_id, "agent-1");
    assert_eq!(result.platform, "linux-tpm2");
}