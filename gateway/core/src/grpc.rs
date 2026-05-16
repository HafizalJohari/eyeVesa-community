use crate::proto;
use crate::proto::gateway_service_client::GatewayServiceClient;
use tonic::transport::Channel;

pub struct ControlPlaneClient {
    client: GatewayServiceClient<Channel>,
}

impl ControlPlaneClient {
    pub async fn connect(addr: &str) -> Result<Self, tonic::transport::Error> {
        let client = GatewayServiceClient::connect(addr.to_string()).await?;
        Ok(Self { client })
    }

    pub async fn register_agent(
        &mut self,
        name: String,
        owner: String,
        capabilities: Vec<String>,
        allowed_tools: Vec<String>,
        max_budget_usd: f64,
    ) -> Result<proto::RegisterAgentResponse, tonic::Status> {
        let request = tonic::Request::new(proto::RegisterAgentRequest {
            name,
            owner,
            capabilities,
            allowed_tools,
            max_budget_usd,
            delegation_policy: "no_chain".to_string(),
            behavioral_tags: vec![],
        });
        let response = self.client.register_agent(request).await?;
        Ok(response.into_inner())
    }

    pub async fn authorize(
        &mut self,
        agent_id: String,
        resource_id: String,
        action: String,
        params_json: String,
    ) -> Result<proto::AuthorizeResponse, tonic::Status> {
        let request = tonic::Request::new(proto::AuthorizeRequest {
            agent_id,
            resource_id,
            action,
            params_json,
        });
        let response = self.client.authorize(request).await?;
        Ok(response.into_inner())
    }

    #[allow(dead_code)]
    pub async fn verify_signature(
        &mut self,
        agent_id: String,
        message: Vec<u8>,
        signature: Vec<u8>,
    ) -> Result<proto::VerifySignatureResponse, tonic::Status> {
        let request = tonic::Request::new(proto::VerifySignatureRequest {
            agent_id,
            message,
            signature,
        });
        let response = self.client.verify_signature(request).await?;
        Ok(response.into_inner())
    }

    #[allow(dead_code)]
    pub async fn get_agent(
        &mut self,
        agent_id: String,
    ) -> Result<proto::GetAgentResponse, tonic::Status> {
        let request = tonic::Request::new(proto::GetAgentRequest { agent_id });
        let response = self.client.get_agent(request).await?;
        Ok(response.into_inner())
    }
}