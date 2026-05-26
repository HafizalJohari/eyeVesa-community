package tui

import (
	"fmt"
	"strings"
)

// ProcessCommand handles command execution dynamically based on active TUI state
func ProcessCommand(m model, cmd string) string {
	cmd = strings.TrimSpace(cmd)
	lowerCmd := strings.ToLower(cmd)
	parts := strings.Fields(cmd)

	// ── Agents ──────────────────────────────────────────────────────────────
	if strings.HasPrefix(lowerCmd, "check agent ") {
		agentName := strings.TrimSpace(cmd[12:])
		for _, a := range m.agents {
			if strings.EqualFold(a.Name, agentName) {
				return fmt.Sprintf("Agent: %s\nDID: %s\nTrust Score: %d%%\nStatus: %s",
					a.Name, a.DID, a.Trust, a.Status)
			}
		}
		return fmt.Sprintf("Agent '%s' not found in registry.", agentName)
	}

	if lowerCmd == "list agents" || lowerCmd == "agents" {
		if len(m.agents) == 0 {
			return "No agents registered."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Agents (%d):\n", len(m.agents)))
		for _, a := range m.agents {
			sb.WriteString(fmt.Sprintf("  %-14s  %-24s  trust:%d%%  %s\n", a.Name, a.DID, a.Trust, a.Status))
		}
		return sb.String()
	}

	// ── HITL ────────────────────────────────────────────────────────────────
	if lowerCmd == "hitl list" || lowerCmd == "hitl pending" || lowerCmd == "hitl" {
		if len(m.hitlRequests) == 0 {
			return "No pending HITL requests."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("HITL Pending (%d):\n", len(m.hitlRequests)))
		for _, r := range m.hitlRequests {
			sb.WriteString(fmt.Sprintf("  [%s] %-12s  %-14s  risk:%s  %s\n",
				r.ID, r.AgentID, r.Action, r.RiskLevel, r.Status))
		}
		return sb.String()
	}

	if len(parts) >= 3 && lowerCmd[:len("hitl approve")] == "hitl approve" {
		id := parts[2]
		res, err := m.apiClient.ApproveHILT(id, "tui")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		status, _ := res["status"].(string)
		return fmt.Sprintf("HITL %s APPROVED. status=%s", id, status)
	}

	if len(parts) >= 3 && strings.HasPrefix(lowerCmd, "hitl deny") {
		id := parts[2]
		res, err := m.apiClient.DenyHILT(id, "tui")
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		status, _ := res["status"].(string)
		return fmt.Sprintf("HITL %s DENIED. status=%s", id, status)
	}

	// ── Airport ─────────────────────────────────────────────────────────────
	if lowerCmd == "airport" || lowerCmd == "airport online" || lowerCmd == "airport search" {
		if len(m.airportAgents) == 0 {
			return "No agents visible at the airport."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Airport Agents (%d):\n", len(m.airportAgents)))
		for _, a := range m.airportAgents {
			sb.WriteString(fmt.Sprintf("  %-14s  %-8s  trust:%.0f%%  %s\n",
				a.Name, a.Status, a.TrustScore*100, a.Tags))
		}
		return sb.String()
	}

	if len(parts) >= 3 && parts[0] == "airport" && parts[1] == "heartbeat" {
		agentID := parts[2]
		res, err := m.apiClient.AirportHeartbeat(agentID, "online")
		if err != nil {
			return fmt.Sprintf("ERROR heartbeat: %v", err)
		}
		msg, _ := res["message"].(string)
		return fmt.Sprintf("Heartbeat sent for %s. %s", agentID, msg)
	}

	if len(parts) >= 3 && parts[0] == "airport" && parts[1] == "profile" {
		agentID := parts[2]
		res, err := m.apiClient.AirportGetProfile(agentID)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		name, _ := res["name"].(string)
		status, _ := res["status"].(string)
		return fmt.Sprintf("Profile [%s]: name=%s status=%s", agentID, name, status)
	}

	// ── Skills ───────────────────────────────────────────────────────────────
	if lowerCmd == "skills" || lowerCmd == "skills list" {
		if len(m.skills) == 0 {
			return "No skills in catalog."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Skills Catalog (%d):\n", len(m.skills)))
		for _, sk := range m.skills {
			sb.WriteString(fmt.Sprintf("  %-14s  cat:%-14s  risk:%-8s  trustMin:%s\n",
				sk.Name, sk.Category, sk.RiskLevel, sk.TrustMin))
		}
		return sb.String()
	}

	if len(parts) >= 3 && parts[0] == "skills" && parts[1] == "search" {
		query := strings.Join(parts[2:], " ")
		var sb strings.Builder
		for _, sk := range m.skills {
			if strings.Contains(strings.ToLower(sk.Name), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(sk.Category), strings.ToLower(query)) {
				sb.WriteString(fmt.Sprintf("  %-14s  %s  risk:%s\n", sk.Name, sk.Category, sk.RiskLevel))
			}
		}
		if sb.Len() == 0 {
			return fmt.Sprintf("No skills matching '%s'.", query)
		}
		return "Matching skills:\n" + sb.String()
	}

	if len(parts) >= 4 && parts[0] == "assign" && parts[1] == "skill" {
		agentID := parts[2]
		skillID := parts[3]
		res, err := m.apiClient.AssignSkill(agentID, skillID, 1)
		if err != nil {
			return fmt.Sprintf("ERROR assigning skill: %v", err)
		}
		msg, _ := res["message"].(string)
		return fmt.Sprintf("Skill '%s' assigned to '%s'. %s", skillID, agentID, msg)
	}

	// ── Audit ─────────────────────────────────────────────────────────────────
	if lowerCmd == "audit" || lowerCmd == "view audit trail" {
		if len(m.auditLogs) == 0 {
			return "No audit logs."
		}
		var sb strings.Builder
		sb.WriteString("Recent Audit Trail:\n")
		limit := 6
		if len(m.auditLogs) < limit {
			limit = len(m.auditLogs)
		}
		for i := 0; i < limit; i++ {
			log := m.auditLogs[i]
			sb.WriteString(fmt.Sprintf("  [%s] %-10s  %s\n", log.Timestamp, log.Agent, log.Event))
		}
		return sb.String()
	}

	if len(parts) >= 2 && parts[0] == "audit" {
		agentID := parts[1]
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Audit logs for agent '%s':\n", agentID))
		for _, log := range m.auditLogs {
			if strings.Contains(strings.ToLower(log.Agent), strings.ToLower(agentID)) {
				sb.WriteString(fmt.Sprintf("  [%s] %s\n", log.Timestamp, log.Event))
			}
		}
		return sb.String()
	}

	// ── Status ────────────────────────────────────────────────────────────────
	if lowerCmd == "status" || lowerCmd == "system status" {
		var sb strings.Builder
		sb.WriteString("System Services:\n")
		for _, s := range m.svcStatus {
			sb.WriteString(fmt.Sprintf("  %-18s  %s  uptime:%s\n", s.Name, s.Status, s.Uptime))
		}
		return sb.String()
	}

	// ── Authorize ─────────────────────────────────────────────────────────────
	if len(parts) >= 3 && parts[0] == "authorize" {
		agentID := parts[1]
		action := parts[2]
		resourceID := ""
		if len(parts) >= 4 {
			resourceID = parts[3]
		}
		res, err := m.apiClient.Authorize(agentID, action, resourceID, nil)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		decision, _ := res["decision"].(string)
		reason, _ := res["reason"].(string)
		return fmt.Sprintf("Authorize [%s → %s]: decision=%s  reason=%s", agentID, action, decision, reason)
	}

	// ── Simulate ────────────────────────────────────────────────────────────
	if lowerCmd == "simulate deploy_request" {
		agentName := "hermes-ops"
		if len(m.agents) > 0 {
			agentName = m.agents[0].Name
		}
		return fmt.Sprintf("Action: k8s_deploy\nAgent: %s\nPolicy: requires_human_approval\nDecision: PENDING_APPROVAL (HITL Request dispatched)\nAudit: event cryptographically recorded", agentName)
	}

	// ── Federation ────────────────────────────────────────────────────────────
	if lowerCmd == "federation" || lowerCmd == "federation list" {
		if len(m.trustBundles) == 0 {
			return "No trust bundles registered."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Trust Bundles (%d):\n", len(m.trustBundles)))
		for _, b := range m.trustBundles {
			sb.WriteString(fmt.Sprintf("  %-18s  %-12s  federated:%-4s  %s\n",
				b.TrustDomain, b.Type, b.Federated, b.Status))
		}
		return sb.String()
	}

	if len(parts) >= 3 && parts[0] == "federation" && parts[1] == "verify" {
		domain := parts[2]
		res, err := m.apiClient.VerifyTrustBundle(domain)
		if err != nil {
			return fmt.Sprintf("ERROR: %v", err)
		}
		status, _ := res["status"].(string)
		return fmt.Sprintf("Trust bundle '%s' verify: status=%s", domain, status)
	}

	// ── API Keys ──────────────────────────────────────────────────────────────
	if lowerCmd == "apikeys" || lowerCmd == "apikeys list" || lowerCmd == "api-keys" {
		if len(m.apiKeys) == 0 {
			return "No API keys."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("API Keys (%d):\n", len(m.apiKeys)))
		for _, k := range m.apiKeys {
			sb.WriteString(fmt.Sprintf("  %-14s  %-10s  tenant:%-12s  %s  %s\n",
				k.Name, k.ID, k.TenantID, k.Status, k.CreatedAt))
		}
		return sb.String()
	}

	if len(parts) >= 3 && parts[0] == "apikeys" && parts[1] == "create" {
		name := parts[2]
		tenant := ""
		if len(parts) >= 4 {
			tenant = parts[3]
		}
		res, err := m.apiClient.CreateAPIKey(name, tenant)
		if err != nil {
			return fmt.Sprintf("ERROR creating API key: %v", err)
		}
		id, _ := res["id"].(string)
		return fmt.Sprintf("API key created: name=%s  id=%s", name, id)
	}

	if len(parts) >= 3 && parts[0] == "apikeys" && parts[1] == "revoke" {
		keyID := parts[2]
		_, err := m.apiClient.RevokeAPIKey(keyID)
		if err != nil {
			return fmt.Sprintf("ERROR revoking key: %v", err)
		}
		return fmt.Sprintf("API key '%s' revoked.", keyID)
	}

	// ── Security ──────────────────────────────────────────────────────────────
	if lowerCmd == "security" || lowerCmd == "security status" {
		if len(m.securityEvents) == 0 {
			return "No security scan data. Set GITHUB_TOKEN env var and press [r] to refresh."
		}
		var sb strings.Builder
		sb.WriteString("Security CI/CD Scans:\n")
		for _, e := range m.securityEvents {
			result := strings.ToUpper(e.Conclusion)
			sb.WriteString(fmt.Sprintf("  %-28s  %-9s  branch:%s  %s\n",
				e.Workflow, result, e.Branch, e.RunAt))
		}
		return sb.String()
	}

	// ── Help / Clear ─────────────────────────────────────────────────────────

	if lowerCmd == "help" {
		return `Available Commands:
  check agent <name>           Agent details from registry
  list agents                  List all registered agents
  hitl list                    List pending HITL approvals
  hitl approve <id>            Approve a HITL request
  hitl deny <id>               Deny a HITL request
  airport                      List agents at the Airport
  airport heartbeat <id>       Send agent heartbeat
  airport profile <id>         Get airport profile
  skills list                  List skills catalog
  skills search <query>        Search skills
  assign skill <agent> <skill> Assign skill to agent
  audit                        View recent audit trail
  audit <agent>                Audit logs for specific agent
  status                       System service status
  authorize <agent> <action>   Policy authorization check
  federation list              List SPIRE trust bundles
  federation verify <domain>   Verify trust bundle integrity
  apikeys list                 List API keys
  apikeys create <name>        Create API key
  apikeys revoke <id>          Revoke API key
  security                     Show CI/CD security scan results
  simulate deploy_request      Demo OPA policy engine
  clear                        Clear output
  q / quit                     Exit TUI`
	}

	if lowerCmd == "clear" {
		return ""
	}

	if lowerCmd == "q" || lowerCmd == "quit" {
		return "__QUIT__"
	}

	return fmt.Sprintf("Unknown command: '%s'. Type 'help' for available commands.", cmd)
}
