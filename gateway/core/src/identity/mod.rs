pub mod ptv;
pub mod svid;

use serde::{Deserialize, Serialize};

/// Prove-Transform-Verify (PTV) Protocol
///
/// Binds agent identities to hardware roots of trust (TPM 2.0 / Secure Enclave).
/// Three phases:
/// 1. PROVE: Agent attests its runtime environment using a hardware-bound key
/// 2. TRANSFORM: Gateway verifies the attestation and binds it to the agent's identity
/// 3. VERIFY: Any party can later verify that the binding is still valid

#[derive(Debug, Clone, Serialize, Deserialize)]
#[allow(dead_code)]
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
#[allow(dead_code)]
pub struct AttestationProof {
    pub attestation: HardwareAttestation,
    pub tpm_signature: Vec<u8>,
    pub quote: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[allow(dead_code)]
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
#[allow(dead_code)]
pub struct VerificationResult {
    pub valid: bool,
    pub agent_id: String,
    pub platform: String,
    pub message: String,
    pub verified_at: i64,
}