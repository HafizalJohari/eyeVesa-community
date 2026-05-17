pub mod ptv;
pub mod svid;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HardwareAttestation {
    pub agent_id: String,
    pub platform: String,
    pub firmware_version: String,
    pub tpm_public_key: Vec<u8>,
    pub runtime_hash: Vec<u8>,
    pub timestamp: i64,
    pub nonce: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AttestationProof {
    pub attestation: HardwareAttestation,
    pub tpm_signature: Vec<u8>,
    pub quote: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IdentityBinding {
    pub binding_id: String,
    pub agent_id: String,
    pub agent_public_key: Vec<u8>,
    pub hardware_public_key: Vec<u8>,
    pub platform: String,
    pub runtime_hash: Vec<u8>,
    pub transformed_at: i64,
    pub binding_signature: Vec<u8>,
    pub expires_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VerificationResult {
    pub valid: bool,
    pub agent_id: String,
    pub platform: String,
    pub message: String,
    pub verified_at: i64,
}