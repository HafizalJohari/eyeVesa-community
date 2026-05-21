package agentid.spire

import rego.v1

default allow := false

allow if {
    input.action.method == "GET"
    spire_read_permitted
}

allow if {
    input.action.method == "POST"
    spire_write_permitted
}

allow if {
    input.action.method == "PUT"
    spire_write_permitted
}

allow if {
    input.action.method == "DELETE"
    spire_admin_permitted
}

spire_read_permitted if {
    input.agent.trust_score >= 0.3
    valid_agent
}

spire_write_permitted if {
    input.agent.trust_score >= 0.6
    valid_agent
    input.action.category == "bundle_management"
    requires_hitl_check
}

spire_write_permitted if {
    input.agent.trust_score >= 0.7
    valid_agent
    input.action.category == "workload_registration"
}

spire_admin_permitted if {
    input.agent.trust_score >= 0.9
    valid_agent
    input.action.category == "bundle_management"
}

spire_admin_permitted if {
    input.agent.trust_score >= 0.8
    valid_agent
    input.action.category == "workload_registration"
}

valid_agent if {
    input.agent.id
    input.agent.status == "active"
    input.agent.trust_score >= 0.1
}

requires_hitl_check if {
    input.action.category == "bundle_management"
    input.action.method == "POST"
    not same_trust_domain
}

same_trust_domain if {
    input.agent.trust_domain == input.action.trust_domain
}

federation_write_allowed if {
    input.agent.trust_score >= 0.8
    input.action.is_federated == true
}

bundle_verification_required if {
    input.action.method == "POST"
    input.action.category == "bundle_management"
    not same_trust_domain
}

reason := "spire read permitted" if {
    spire_read_permitted
    not spire_write_permitted
}

reason := "spire write permitted" if {
    spire_write_permitted
    not spire_admin_permitted
}

reason := "spire admin permitted" if {
    spire_admin_permitted
}

reason := "insufficient trust score for spire operation" if {
    not spire_read_permitted
    not spire_write_permitted
    not spire_admin_permitted
}

reason := "spire write requires HITL for cross-domain bundle" if {
    spire_write_permitted
    bundle_verification_required
}

trust_delta := 0.02 if {
    allow
    input.action.method == "GET"
}

trust_delta := 0.0 if {
    allow
    input.action.method != "GET"
}

trust_delta := -0.1 if {
    not allow
}

risk_level := "low" if {
    input.action.method == "GET"
    allow
}

risk_level := "medium" if {
    input.action.category == "workload_registration"
    allow
}

risk_level := "high" if {
    input.action.category == "bundle_management"
    input.action.method != "GET"
    allow
}

risk_level := "critical" if {
    input.action.is_federated == true
    input.action.method != "GET"
    allow
}

risk_level := "critical" if {
    not allow
}

requires_hitl := true if {
    bundle_verification_required
    allow
}

requires_hitl := false if {
    not bundle_verification_required
}

requires_escalation := true if {
    input.action.is_federated == true
    input.action.method == "DELETE"
    allow
}

requires_escalation := false if {
    not input.action.is_federated
}

requires_escalation := false if {
    input.action.method != "DELETE"
}