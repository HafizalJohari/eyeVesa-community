package agentid.federation

import future.keywords

# Federation authorization policies
# Controls which gateways can register, which agents can sync, and cross-gateway actions

# Default deny
default allow = false
default requires_hitl = false
default trust_delta = 0.0

# Peer gateway registration rules
peer_registration_allowed {
	input.action == "federation.register"
	input.peer.trust_score >= 0.0
}

peer_registration_requires_hitl {
	input.action == "federation.register"
	input.peer.trust_score < 0.5
}

# Agent sync rules (gateway presents passport)
agent_sync_allowed {
	input.action == "federation.sync"
	input.gateway.status == "active"
	input.gateway.trust_score >= 0.5
	input.passport.valid == true
}

agent_sync_requires_hitl {
	input.action == "federation.sync"
	input.gateway.trust_score < 0.7
}

# Federated heartbeat — always allowed for registered agents
heartbeat_allowed {
	input.action == "federation.heartbeat"
	input.gateway.status == "active"
}

# Cross-gateway invoke
cross_gateway_invoke_allowed {
	input.action == "federation.invoke"
	input.requester.gateway.status == "active"
	input.responder.gateway.status == "active"
	input.requester.trust_score >= 0.5
	input.responder.trust_score >= 0.3
}

cross_gateway_invoke_requires_hitl {
	input.action == "federation.invoke"
	input.requester.trust_score < 0.7
}

cross_gateway_invoke_risk_level = "high" {
	input.action == "federation.invoke"
}

# Peer suspension — requires high trust
peer_suspend_allowed {
	input.action == "federation.suspend"
	input.actor.trust_score >= 0.9
}

peer_suspend_requires_hitl {
	input.action == "federation.suspend"
}

# Main allow rule
allow {
	peer_registration_allowed
}
allow {
	agent_sync_allowed
}
allow {
	heartbeat_allowed
}
allow {
	cross_gateway_invoke_allowed
}
allow {
	peer_suspend_allowed
}

# HITL requirements
requires_hitl {
	peer_registration_requires_hitl
}
requires_hitl {
	agent_sync_requires_hitl
}
requires_hitl {
	cross_gateway_invoke_requires_hitl
}
requires_hitl {
	peer_suspend_requires_hitl
}

# Trust deltas
trust_delta = 0.01 {
	input.action == "federation.sync"
	input.gateway.trust_score >= 0.8
}
trust_delta = -0.05 {
	input.action == "federation.sync"
	not input.passport.valid
}
trust_delta = -0.1 {
	input.action == "federation.invoke"
	not cross_gateway_invoke_allowed
}

# Reason strings
reason = "peer gateway trust score too low" {
	not peer_registration_allowed
	input.action == "federation.register"
}
reason = "gateway not active or passport invalid" {
	not agent_sync_allowed
	input.action == "federation.sync"
}
reason = "gateway not active" {
	not heartbeat_allowed
	input.action == "federation.heartbeat"
}
reason = "cross-gateway invoke denied: trust or gateway status insufficient" {
	not cross_gateway_invoke_allowed
	input.action == "federation.invoke"
}
reason = "insufficient trust to suspend peer" {
	not peer_suspend_allowed
	input.action == "federation.suspend"
}
