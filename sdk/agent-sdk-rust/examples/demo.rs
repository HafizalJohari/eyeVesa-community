use agentid_sdk::client::AgentClient;
use agentid_sdk::AgentConfig;
use rand::rngs::OsRng;
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let agent_id = Uuid::new_v4();
    let signing_key = ed25519_dalek::SigningKey::generate(&mut OsRng);

    let config = AgentConfig {
        agent_id,
        name: "sdk-demo-agent".to_string(),
        owner: "demo-team".to_string(),
        gateway_endpoint: "http://localhost:9443".to_string(),
    };

    println!("=== AgentID SDK Demo ===\n");
    println!("Agent ID: {}", agent_id);
    println!("Connecting to gateway at {}...\n", config.gateway_endpoint);

    // Step 1: Connect (register)
    let client = AgentClient::connect(config, signing_key).await?;
    println!("Connected! Registered: {}", client.is_registered());
    println!("Trust score: {:.2}\n", client.trust_score());

    // Step 2: MCP Initialize
    println!("--- MCP Initialize ---");
    match client.mcp_initialize().await {
        Ok(caps) => {
            println!("Protocol: {}", caps.protocol_version);
            println!("Tools: {}, Resources: {}, Prompts: {}\n", caps.tools, caps.resources, caps.prompts);
        }
        Err(e) => println!("MCP init error: {}\n", e),
    }

    // Step 3: Discover resources
    println!("--- Discovering resources ---");
    match client.discover("mcp").await {
        Ok(tools) => {
            for tool in &tools {
                println!("Found tool: {} - {}", tool.name, tool.description);
            }
            println!();
        }
        Err(e) => println!("Discovery: {}\n", e),
    }

    // Step 4: Authorize & Invoke
    let resource_id = Uuid::parse_str("d4385f9f-bcf8-47b9-90f4-b1fce91def59").unwrap();

    println!("--- Invoking read ---");
    match client.invoke(&resource_id, "read", serde_json::json!({"location": "Kuala Lumpur"})).await {
        Ok(result) => {
            println!("Success: {:?}", result.data);
            println!("Trust score after: {:.2}\n", result.trust_score);
        }
        Err(e) => println!("Error: {}\n", e),
    }

    println!("--- Invoking delete (should be denied) ---");
    match client.invoke(&resource_id, "delete", serde_json::json!({})).await {
        Ok(result) => println!("Unexpectedly allowed: {:?}", result.data),
        Err(e) => println!("Denied (expected): {}\n", e),
    }

    // Step 5: HITL Approval
    println!("--- Requesting HITL approval ---");
    match client.request_approval("bank_transfer", "Transfer $10K externally", "high").await {
        Ok(approval) => {
            println!("Approval requested: {} (status: {})", approval.approval_id, approval.status);
        }
        Err(e) => println!("HITL request error: {}", e),
    }

    // Step 6: PTV Attestation
    println!("\n--- PTV Attestation ---");
    match client.attest("linux-tpm2", "2.0.0").await {
        Ok(result) => println!("Attestation successful, quote length: {}", result.quote.len()),
        Err(e) => println!("Attestation error: {}", e),
    }

    println!("\nSDK demo complete!");
    Ok(())
}