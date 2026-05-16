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

    let client = AgentClient::connect(config, signing_key).await?;
    println!("Connected! Registered: {}", client.is_registered());
    println!("Trust score: {:.2}\n", client.trust_score());

    let resource_id = Uuid::parse_str("d4385f9f-bcf8-47b9-90f4-b1fce91def59").unwrap();

    println!("--- Invoking get_weather ---");
    match client.invoke(&resource_id, "read", serde_json::json!({"location": "Kuala Lumpur"})).await {
        Ok(result) => {
            println!("Success: {:?}", result.data);
            println!("Trust score after: {:.2}\n", result.trust_score);
        }
        Err(e) => println!("Error: {}\n", e),
    }

    println!("--- Invoking delete (should be denied) ---");
    match client.invoke(&resource_id, "delete", serde_json::json!({})).await {
        Ok(result) => {
            println!("Success: {:?}", result.data);
        }
        Err(e) => println!("Denied (expected): {}\n", e),
    }

    println!("--- Discovering resources ---");
    match client.discover("mcp").await {
        Ok(tools) => {
            for tool in &tools {
                println!("Found tool: {} - {}", tool.name, tool.description);
            }
        }
        Err(e) => println!("Discovery: {}", e),
    }

    println!("\nSDK demo complete!");
    Ok(())
}