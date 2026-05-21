pub mod crypto;
pub mod grpc;
pub mod identity;
pub mod proxy;
pub mod tls;

pub mod proto {
    tonic::include_proto!("agentid");
}