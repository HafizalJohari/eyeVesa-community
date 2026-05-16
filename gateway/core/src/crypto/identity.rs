use ed25519_dalek::{SigningKey, VerifyingKey, Signer, Verifier, Signature};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[allow(dead_code)]
pub struct AgentIdentity {
    pub agent_id: Uuid,
    pub owner: String,
    pub public_key: Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[allow(dead_code)]
pub struct AgentRegistration {
    pub name: String,
    pub owner: String,
    pub capabilities: Vec<String>,
    pub allowed_tools: Vec<String>,
    pub max_budget_usd: f64,
}

#[allow(dead_code)]
pub fn generate_agent_keypair() -> (SigningKey, VerifyingKey) {
    let signing_key = SigningKey::generate(&mut rand::rngs::OsRng);
    let verifying_key = signing_key.verifying_key();
    (signing_key, verifying_key)
}

#[allow(dead_code)]
pub fn sign_agent_request(
    signing_key: &SigningKey,
    message: &[u8],
) -> Signature {
    signing_key.sign(message)
}

#[allow(dead_code)]
pub fn verify_agent_signature(
    verifying_key: &VerifyingKey,
    message: &[u8],
    signature: &Signature,
) -> bool {
    verifying_key.verify(message, signature).is_ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_keypair_sign_verify() {
        let (signing_key, verifying_key) = generate_agent_keypair();
        let message = b"test-agent-request";
        let signature = sign_agent_request(&signing_key, message);
        assert!(verify_agent_signature(&verifying_key, message, &signature));
    }

    #[test]
    fn test_invalid_signature_fails() {
        let (signing_key, _) = generate_agent_keypair();
        let (_, wrong_verifying_key) = generate_agent_keypair();
        let message = b"test-agent-request";
        let signature = sign_agent_request(&signing_key, message);
        assert!(!verify_agent_signature(&wrong_verifying_key, message, &signature));
    }
}