package a2a

func ToAgentCard(row map[string]interface{}) AgentCard {
	card := AgentCard{}
	if v, ok := row["agent_id"].(string); ok {
		card.ID = v
	}
	if v, ok := row["name"].(string); ok {
		card.Name = v
	}
	if v, ok := row["owner"].(string); ok {
		card.Owner = v
	}
	if v, ok := row["status"].(string); ok {
		card.Status = v
	}
	if v, ok := row["trust_score"].(float64); ok {
		card.TrustScore = v
	}
	if v, ok := row["capabilities"].([]string); ok {
		card.Capabilities = v
	}
	return card
}
