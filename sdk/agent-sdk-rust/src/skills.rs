use crate::client::AgentClient;
use crate::ToolInfo;
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, thiserror::Error)]
pub enum SkillError {
    #[error("Skill not found: {0}")]
    NotFound(String),
    #[error("Gateway error: {0}")]
    Gateway(String),
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Skill {
    pub skill_id: String,
    pub name: String,
    pub description: String,
    pub category: String,
    pub risk_level: String,
    pub required_trust_min: f64,
    pub required_proficiency: i32,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentSkill {
    pub agent_id: String,
    pub skill_id: String,
    #[serde(default)]
    pub skill_name: String,
    pub proficiency: i32,
    pub verified: bool,
    #[serde(default)]
    pub verified_by: String,
    #[serde(default)]
    pub verified_at: Option<String>,
    pub endorsements_count: i32,
    pub acquired_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Endorsement {
    pub endorsement_id: String,
    pub agent_id: String,
    pub skill_id: String,
    pub endorser_type: String,
    pub endorser_id: String,
    #[serde(default)]
    pub comment: String,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillTrustScore {
    pub agent_id: String,
    pub skill_id: String,
    #[serde(default)]
    pub skill_name: String,
    pub trust_score: f64,
    pub updated_at: String,
}

#[derive(Debug, Serialize)]
struct CreateSkillRequest {
    name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    description: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    category: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    risk_level: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    required_trust_min: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    required_proficiency: Option<i32>,
}

#[derive(Debug, Serialize)]
struct AssignSkillRequest {
    skill_id: String,
    proficiency: i32,
}

#[derive(Debug, Serialize)]
struct EndorseRequest {
    endorser_type: String,
    endorser_id: String,
    comment: String,
}

#[derive(Debug, Serialize)]
struct VerifyRequest {
    verified_by: String,
}

#[derive(Debug, Serialize)]
struct TrustAdjustRequest {
    delta: f64,
    reason: String,
}

#[derive(Debug, Deserialize)]
struct SkillsListResponse {
    #[serde(default)]
    skills: Vec<Skill>,
}

#[derive(Debug, Deserialize)]
struct AgentSkillsResponse {
    #[serde(default)]
    skills: Vec<AgentSkill>,
}

#[derive(Debug, Deserialize)]
struct EndorsementsResponse {
    #[serde(default)]
    endorsements: Vec<Endorsement>,
}

impl AgentClient {
    pub async fn list_skills(&self, category: &str) -> Result<Vec<Skill>, SkillError> {
        let gateway = self.gateway_endpoint();
        let mut url = format!("{}/v1/skills", gateway.trim_end_matches('/'));
        if !category.is_empty() {
            url = format!("{}?category={}", url, category);
        }

        let resp = self.http_client().get(&url).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("list skills failed: {}", resp.status())));
        }

        let body: Value = resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))?;
        let skills: Vec<Skill> = serde_json::from_value(
            body.get("skills").cloned().unwrap_or(Value::Array(vec![]))
        ).unwrap_or_default();

        Ok(skills)
    }

    pub async fn create_skill(&self, name: &str, description: &str, category: &str, risk_level: &str, required_trust_min: f64, required_proficiency: i32) -> Result<Skill, SkillError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/skills", gateway.trim_end_matches('/'));

        let req = CreateSkillRequest {
            name: name.to_string(),
            description: description.to_string(),
            category: category.to_string(),
            risk_level: risk_level.to_string(),
            required_trust_min: if required_trust_min > 0.0 { Some(required_trust_min) } else { None },
            required_proficiency: if required_proficiency > 0 { Some(required_proficiency) } else { None },
        };

        let resp = self.http_client().post(&url).json(&req).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("create skill failed: {}", resp.status())));
        }

        resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))
    }

    pub async fn assign_skill(&self, agent_id: &str, skill_id: &str, proficiency: i32) -> Result<AgentSkill, SkillError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/agents/{}/skills", gateway.trim_end_matches('/'), agent_id);

        let req = AssignSkillRequest {
            skill_id: skill_id.to_string(),
            proficiency,
        };

        let resp = self.http_client().post(&url).json(&req).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("assign skill failed: {}", resp.status())));
        }

        resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))
    }

    pub async fn endorse_skill(&self, agent_id: &str, skill_id: &str, endorser_type: &str, endorser_id: &str, comment: &str) -> Result<Endorsement, SkillError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/agents/{}/skills/{}/endorse", gateway.trim_end_matches('/'), agent_id, skill_id);

        let req = EndorseRequest {
            endorser_type: endorser_type.to_string(),
            endorser_id: endorser_id.to_string(),
            comment: comment.to_string(),
        };

        let resp = self.http_client().post(&url).json(&req).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("endorse skill failed: {}", resp.status())));
        }

        resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))
    }

    pub async fn verify_skill(&self, agent_id: &str, skill_id: &str, verified_by: &str) -> Result<AgentSkill, SkillError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/agents/{}/skills/{}/verify", gateway.trim_end_matches('/'), agent_id, skill_id);

        let req = VerifyRequest {
            verified_by: verified_by.to_string(),
        };

        let resp = self.http_client().post(&url).json(&req).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("verify skill failed: {}", resp.status())));
        }

        resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))
    }

    pub async fn get_skill_trust(&self, agent_id: &str) -> Result<Vec<SkillTrustScore>, SkillError> {
        let gateway = self.gateway_endpoint();
        let url = format!("{}/v1/agents/{}/skill-trust", gateway.trim_end_matches('/'), agent_id);

        let resp = self.http_client().get(&url).send().await?;
        if !resp.status().is_success() {
            return Err(SkillError::Gateway(format!("get skill trust failed: {}", resp.status())));
        }

        let body: Value = resp.json().await.map_err(|e| SkillError::Gateway(e.to_string()))?;
        let scores: Vec<SkillTrustScore> = serde_json::from_value(
            body.get("scores").cloned().unwrap_or(Value::Array(vec![]))
        ).unwrap_or_default();

        Ok(scores)
    }
}