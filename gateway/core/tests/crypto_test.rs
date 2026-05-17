use ed25519_dalek::{SigningKey, Signer, Verifier, VerifyingKey};
use rand::rngs::OsRng;
use sha2::{Sha256, Digest};

fn generate_keypair() -> (SigningKey, VerifyingKey) {
    let signing_key = SigningKey::generate(&mut OsRng);
    let verifying_key = signing_key.verifying_key();
    (signing_key, verifying_key)
}

#[test]
fn test_ed25519_sign_verify() {
    let (signing_key, verifying_key) = generate_keypair();
    let message = b"test-message";
    let signature = signing_key.sign(message);
    assert!(verifying_key.verify(message, &signature).is_ok());
}

#[test]
fn test_ed25519_wrong_key_fails() {
    let (signing_key, _) = generate_keypair();
    let (_, wrong_verifying_key) = generate_keypair();
    let message = b"test-message";
    let signature = signing_key.sign(message);
    assert!(wrong_verifying_key.verify(message, &signature).is_err());
}

#[test]
fn test_ed25519_different_messages() {
    let (signing_key, verifying_key) = generate_keypair();
    let msg1 = b"message-1";
    let msg2 = b"message-2";
    let sig1 = signing_key.sign(msg1);
    assert!(verifying_key.verify(msg1, &sig1).is_ok());
    assert!(verifying_key.verify(msg2, &sig1).is_err());
}

#[test]
fn test_ptv_prove_transform_verify() {
    let tpm_key = SigningKey::generate(&mut OsRng);
    let agent_key = SigningKey::generate(&mut OsRng);

    let nonce = [42u8; 32];
    let runtime_hash = b"sha256:deadbeef";

    let attestation = agentid_core::identity::HardwareAttestation {
        agent_id: "agent-ptv-test".to_string(),
        platform: "test-platform".to_string(),
        firmware_version: "1.0".to_string(),
        tpm_public_key: tpm_key.verifying_key().to_bytes().to_vec(),
        runtime_hash: runtime_hash.to_vec(),
        timestamp: chrono::Utc::now().timestamp(),
        nonce: nonce.to_vec(),
    };

    let attestation_bytes = serde_json::to_vec(&attestation).expect("serialize");
    let tpm_signature = tpm_key.sign(&attestation_bytes);

    let mut hasher = Sha256::new();
    hasher.update(&attestation_bytes);
    hasher.update(tpm_signature.to_bytes());
    let quote = hasher.finalize().to_vec();

    let proof = agentid_core::identity::AttestationProof {
        attestation,
        tpm_signature: tpm_signature.to_bytes().to_vec(),
        quote,
    };

    let binding = agentid_core::identity::ptv::transform(&proof, &agent_key).expect("transform");
    assert_eq!(binding.agent_id, "agent-ptv-test");

    let result = agentid_core::identity::ptv::verify(&binding, &agent_key.verifying_key());
    assert!(result.valid);
}

#[test]
fn test_ptv_verify_wrong_key() {
    let tpm_key = SigningKey::generate(&mut OsRng);
    let agent_key = SigningKey::generate(&mut OsRng);
    let other_key = SigningKey::generate(&mut OsRng);

    let nonce = [42u8; 32];
    let attestation = agentid_core::identity::HardwareAttestation {
        agent_id: "agent-wrong-key".to_string(),
        platform: "test".to_string(),
        firmware_version: "1.0".to_string(),
        tpm_public_key: tpm_key.verifying_key().to_bytes().to_vec(),
        runtime_hash: b"hash".to_vec(),
        timestamp: chrono::Utc::now().timestamp(),
        nonce: nonce.to_vec(),
    };

    let attestation_bytes = serde_json::to_vec(&attestation).expect("serialize");
    let tpm_signature = tpm_key.sign(&attestation_bytes);

    let mut hasher = Sha256::new();
    hasher.update(&attestation_bytes);
    hasher.update(tpm_signature.to_bytes());
    let quote = hasher.finalize().to_vec();

    let proof = agentid_core::identity::AttestationProof {
        attestation,
        tpm_signature: tpm_signature.to_bytes().to_vec(),
        quote,
    };

    let binding = agentid_core::identity::ptv::transform(&proof, &agent_key).expect("transform");
    let result = agentid_core::identity::ptv::verify(&binding, &other_key.verifying_key());
    assert!(!result.valid);
}