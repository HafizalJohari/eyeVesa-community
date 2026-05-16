package agentid.authz

tool_allowed {
    input.action.tool == input.agent.allowed_tools[_]
}

cost_over_budget {
    input.action.estimated_cost
    input.action.estimated_cost > (input.agent.trust_score * 100)
}

allow {
    tool_allowed
    not cost_over_budget
}

requires_hitl {
    not tool_allowed
}
else = true {
    cost_over_budget
}

requires_hitl {
    tool_allowed
    cost_over_budget
}

reason := "tool in allowed list" {
    tool_allowed
    not cost_over_budget
}
else := "estimated cost exceeds trust budget" {
    tool_allowed
    cost_over_budget
}
else := "tool not in agent allowed list" {
    not tool_allowed
}
else := "policy denied"

trust_delta := 0.01 {
    tool_allowed
    not cost_over_budget
}
else := -0.1 {
    tool_allowed
    cost_over_budget
}
else := -0.05 {
    not tool_allowed
}
else := 0.0