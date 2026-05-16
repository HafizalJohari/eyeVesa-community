use ed25519_dalek::{SigningKey, VerifyingKey, Signer, Verifier, Signature};

#[derive(Debug, thiserror::Error)]
pub enum SigningError {
    #[error("Invalid signature")]
    InvalidSignature,
    #[error("Key error: {0}")]
    KeyError(String),
}

pub fn sign(signing_key: &SigningKey, payload: &[u8]) -> Signature {
    signing_key.sign(payload)
}

pub fn verify(
    verifying_key: &VerifyingKey,
    payload: &[u8],
    signature: &Signature,
) -> Result<(), SigningError> {
    verifying_key
        .verify(payload, signature)
        .map_err(|_| SigningError::InvalidSignature)
}