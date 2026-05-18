package agentid.skills

default allow = false
default requires_hitl = true
default skill_allowed = false
default trust_sufficient = false
default reason = "no matching skill found"

skill_allowed {
	input.agent_skills[_].skill_id == input.required_skills[_].skill_id
	input.agent_skills[_].proficiency >= input.required_skills[_].min_proficiency
}

trust_sufficient {
	some i
	input.skill_trust_scores[i].skill_id == input.required_skills[_].skill_id
	input.skill_trust_scores[i].trust_score >= input.required_skills[_].min_trust
}

trust_sufficient {
	not has_skill_trust_score
	input.agent.global_trust >= required_min_trust
}

has_skill_trust_score {
	input.skill_trust_scores[_]
}

required_min_trust = min_val {
	vals := [s.min_trust | s := input.required_skills[_]]
	min_val := min(vals)
}

allow {
	skill_allowed
	trust_sufficient
}

allow {
	count(input.required_skills) == 0
	input.agent.global_trust >= 0.1
}

reason := "skill authorized: proficiency and trust sufficient" {
	allow
	skill_allowed
}

reason := "skill denied: missing required skill" {
	not skill_allowed
	count(input.required_skills) > 0
}

reason := "skill denied: trust below minimum" {
	skill_allowed
	not trust_sufficient
}

trust_delta := 0.01 {
	allow
}

trust_delta := -0.05 {
	not allow
}

trust_delta := 0.00 {
	skill_allowed
	not trust_sufficient
	count(input.required_skills) > 0
}

min_val(arr) = min_val if {
	min_val := min(arr)
}