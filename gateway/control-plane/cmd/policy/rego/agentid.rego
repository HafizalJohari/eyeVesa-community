package agentid.authz

import future.keywords.in
import rego.v1

default allow := false

allow if {
    input.agent.id
    input.action.method
    valid_agent
    action_permitted
    no_never_event
}

valid_agent if {
    agent := data.agents[input.agent.id]
    agent.status == "active"
    agent.trust_score >= 0.1
}

action_permitted if {
    tool_name := input.action.tool
    agent := data.agents[input.agent.id]
    tool_name in agent.allowed_tools
}

never_event_violation if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 500.0
}

no_never_event if {
    not never_event_violation
}

requires_hitl if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 100.0
}

budget_exceeded if {
    agent := data.agents[input.agent.id]
    input.action.estimated_cost > agent.max_budget_usd
}

deny if {
    budget_exceeded
}

deny if {
    never_event_violation
}