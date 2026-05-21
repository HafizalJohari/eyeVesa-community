use crate::identity::{AttestationProof, HardwareAttestation, IdentityBinding, VerificationResult};
use ed25519_dalek::{SigningKey, Signer, Verifier, VerifyingKey};
use rand::rngs::OsRng;
use rand::RngCore;
use sha2::{Sha256, Digest};
use serde_json;

#[allow(dead_code)]
const BINDING_VALIDITY_SECS: i64 = 3600;

#[allow(dead_code)]
pub fn prove(
    agent_id: &str,
    platform: &str,
    firmware_version: &str,
    tpm_signing_key: &SigningKey,
    runtime_hash: &[u8],
    nonce: &[u8],
) -> AttestationProof {
    let timestamp = chrono::Utc::now().timestamp();

    let attestation = HardwareAttestation {
        agent_id: agent_id.to_string(),
        platform: platform.to_string(),
        firmware_version: firmware_version.to_string(),
        tpm_public_key: tpm_signing_key.verifying_key().to_bytes().to_vec(),
        runtime_hash: runtime_hash.to_vec(),
        timestamp,
        nonce: nonce.to_vec(),
    };

    let attestation_bytes = serde_json::to_vec(&attestation)
        .expect("failed to serialize attestation");
    let tpm_signature = tpm_signing_key.sign(&attestation_bytes);

    let mut hasher = Sha256::new();
    hasher.update(&attestation_bytes);
    hasher.update(tpm_signature.to_bytes());
    let quote = hasher.finalize().to_vec();

    AttestationProof {
        attestation,
        tpm_signature: tpm_signature.to_bytes().to_vec(),
        quote,
    }
}

pub fn transform(
    proof: &AttestationProof,
    agent_signing_key: &SigningKey,
) -> Result<IdentityBinding, String> {
    let attestation_bytes = serde_json::to_vec(&proof.attestation)
        .map_err(|e| format!("serialize error: {}", e))?;

    let tpm_pub_bytes: [u8; 32] = proof.attestation.tpm_public_key.clone()
        .try_into()
        .map_err(|_| "TPM public key must be 32 bytes".to_string())?;
    let tpm_verifying_key = VerifyingKey::from_bytes(&tpm_pub_bytes)
        .map_err(|e| format!("invalid TPM key: {}", e))?;

    let tpm_signature = ed25519_dalek::Signature::from_slice(&proof.tpm_signature)
        .map_err(|e| format!("invalid TPM signature: {}", e))?;

    tpm_verifying_key.verify(&attestation_bytes, &tpm_signature)
        .map_err(|e| format!("TPM signature verification failed: {}", e))?;

    let binding_id = uuid::Uuid::new_v4().to_string();
    let timestamp = chrono::Utc::now().timestamp();

    let mut hasher = Sha256::new();
    hasher.update(binding_id.as_bytes());
    hasher.update(proof.attestation.agent_id.as_bytes());
    hasher.update(&proof.attestation.tpm_public_key);
    hasher.update(&proof.attestation.runtime_hash);
    hasher.update(timestamp.to_be_bytes());
    let binding_hash = hasher.finalize();

    let binding_signature = agent_signing_key.sign(&binding_hash);

    Ok(IdentityBinding {
        binding_id,
        agent_id: proof.attestation.agent_id.clone(),
        agent_public_key: agent_signing_key.verifying_key().to_bytes().to_vec(),
        hardware_public_key: proof.attestation.tpm_public_key.clone(),
        platform: proof.attestation.platform.clone(),
        runtime_hash: proof.attestation.runtime_hash.clone(),
        transformed_at: timestamp,
        binding_signature: binding_signature.to_bytes().to_vec(),
        expires_at: timestamp + BINDING_VALIDITY_SECS,
    })
}

pub fn verify(
    binding: &IdentityBinding,
    gateway_verifying_key: &VerifyingKey,
) -> VerificationResult {
    let now = chrono::Utc::now().timestamp();

    if now > binding.expires_at {
        return VerificationResult {
            valid: false,
            agent_id: binding.agent_id.clone(),
            platform: binding.platform.clone(),
            message: "binding has expired".to_string(),
            verified_at: now,
        };
    }

    let mut hasher = Sha256::new();
    hasher.update(binding.binding_id.as_bytes());
    hasher.update(binding.agent_id.as_bytes());
    hasher.update(&binding.hardware_public_key);
    hasher.update(&binding.runtime_hash);
    hasher.update(binding.transformed_at.to_be_bytes());
    let binding_hash = hasher.finalize();

    let binding_signature = match ed25519_dalek::Signature::from_slice(&binding.binding_signature) {
        Ok(s) => s,
        Err(_) => {
            return VerificationResult {
                valid: false,
                agent_id: binding.agent_id.clone(),
                platform: binding.platform.clone(),
                message: "invalid binding signature format".to_string(),
                verified_at: now,
            };
        }
    };

    let valid = gateway_verifying_key.verify(&binding_hash, &binding_signature).is_ok();

    VerificationResult {
        valid,
        agent_id: binding.agent_id.clone(),
        platform: binding.platform.clone(),
        message: if valid {
            "identity binding is valid".to_string()
        } else {
            "binding signature verification failed".to_string()
        },
        verified_at: now,
    }
}

pub fn generate_nonce() -> Vec<u8> {
    let mut nonce = [0u8; 32];
    OsRng.fill_bytes(&mut nonce);
    nonce.to_vec()
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::SigningKey;

    #[test]
    fn test_ptv_full_protocol() {
        let tpm_key = SigningKey::generate(&mut OsRng);
        let agent_key = SigningKey::generate(&mut OsRng);
        let gateway_key = SigningKey::generate(&mut OsRng);

        let nonce = generate_nonce();
        let runtime_hash = b"sha256:abc123def456";

        let proof = prove(
            "agent-test-001",
            "macos-arm64-secure-enclave",
            "1.0.0",
            &tpm_key,
            runtime_hash,
            &nonce,
        );

        let binding = transform(&proof, &agent_key).unwrap();
        assert_eq!(binding.agent_id, "agent-test-001");
        assert_eq!(binding.platform, "macos-arm64-secure-enclave");

        let result = verify(&binding, &gateway_key.verifying_key());
        assert!(!result.valid, "Should fail because gateway key != agent key");

        let result = verify(&binding, &agent_key.verifying_key());
        assert!(result.valid, "Should succeed because agent key signed the binding");
    }

    #[test]
    fn test_expired_binding() {
        let tpm_key = SigningKey::generate(&mut OsRng);
        let agent_key = SigningKey::generate(&mut OsRng);

        let nonce = generate_nonce();
        let proof = prove("agent-expired", "linux-tpm2", "1.0.0", &tpm_key, b"hash", &nonce);
        let mut binding = transform(&proof, &agent_key).unwrap();
        binding.expires_at = chrono::Utc::now().timestamp() - 1;

        let result = verify(&binding, &agent_key.verifying_key());
        assert!(!result.valid);
        assert!(result.message.contains("expired"));
    }

    #[test]
    fn test_tampered_attestation() {
        let tpm_key = SigningKey::generate(&mut OsRng);
        let agent_key = SigningKey::generate(&mut OsRng);

        let nonce = generate_nonce();
        let mut proof = prove("agent-tampered", "linux-tpm2", "1.0.0", &tpm_key, b"hash", &nonce);
        proof.attestation.agent_id = "tampered-agent-id".to_string();

        let result = transform(&proof, &agent_key);
        assert!(result.is_err(), "Should fail because attestation was tampered");
    }
}