package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hafizaljohari/eyeVesa/cli/internal/api"
)

// Active views/panels
type ViewIndex int

const (
	ViewOverview   ViewIndex = iota // 1
	ViewAgents                      // 2
	ViewPolicies                    // 3
	ViewAudit                       // 4
	ViewHITL                        // 5
	ViewAirport                     // 6
	ViewOnboarding                  // 7
	ViewStatus                      // 8
	ViewSkills                      // 9
	ViewFederation                  // 10
	ViewAPIKeys                     // 11
	ViewSecurity                    // 12
	ViewCount      ViewIndex = 12
)

type model struct {
	width            int
	height           int
	currentView      ViewIndex
	showBootSequence bool
	bootStep         int
	bootMessage      string
	spinner          spinner.Model
	commandInput     textinput.Model
	commandOutput    string

	// Live Gateway connection
	apiClient     *api.Client
	connected     bool
	gatewayStatus string

	// Live State data
	agents        []Agent
	policies      []Policy
	auditLogs     []AuditLog
	hitlRequests  []HITLRequest
	airportAgents []AirportAgent
	skills        []Skill
	svcStatus     []ServiceStatus
	trustBundles  []TrustBundle
	apiKeys       []APIKeyEntry
	securityEvents []SecurityEvent

	// Table components
	agentsTable    table.Model
	policiesTable  table.Model
	auditTable     table.Model
	hitlTable      table.Model
	airportTable   table.Model
	skillsTable    table.Model
	statusTable    table.Model
	federationTable table.Model
	apiKeysTable   table.Model
	securityTable  table.Model

	// Onboarding form state
	onboardStep   int
	onboardName   string
	onboardOwner  string
	onboardCaps   string
	onboardResult string
	onboardInput  textinput.Model
}

// InitialModel constructor
func InitialModel() model {
	// 1. Setup Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = BootSpinnerStyle

	// 2. Setup Command Textinput
	ti := textinput.New()
	ti.Placeholder = "press [/] to type a command"
	ti.Blur()
	ti.Prompt = "> "
	ti.PromptStyle = CommandPromptStyle
	ti.TextStyle = CommandTextStyle
	ti.Cursor.Style = CommandCursorStyle
	ti.CharLimit = 120
	ti.Width = 50

	// 3. Onboarding input
	oi := textinput.New()
	oi.Placeholder = "agent name..."
	oi.Prompt = "  name: "
	oi.PromptStyle = CommandPromptStyle
	oi.TextStyle = CommandTextStyle
	oi.CharLimit = 60
	oi.Width = 40

	// 4. Initialize API client to local Gateway control plane
	client := api.NewClient("http://localhost:8080")

	// 5. Setup Tables
	agentT := createAgentsTable(DemoAgents)
	policyT := createPoliciesTable(DemoPolicies)
	auditT := createAuditTable(DemoAuditLogs)
	hitlT := createHITLTable(DemoHITL)
	airportT := createAirportTable(DemoAirport)
	skillsT := createSkillsTable(DemoSkills)
	statusT := createStatusTable(DemoSystemStatus)
	federationT := createFederationTable(DemoTrustBundles)
	apiKeysT := createAPIKeysTable(DemoAPIKeys)
	securityT := createSecurityTable(DemoSecurityEvents)

	return model{
		currentView:      ViewOverview,
		showBootSequence: true,
		bootStep:         0,
		bootMessage:      BootSequenceMessages[0],
		spinner:          s,
		commandInput:     ti,
		onboardInput:     oi,
		apiClient:        client,
		connected:        false,
		gatewayStatus:    "INITIALIZING",
		agents:           DemoAgents,
		policies:         DemoPolicies,
		auditLogs:        DemoAuditLogs,
		hitlRequests:     DemoHITL,
		airportAgents:    DemoAirport,
		skills:           DemoSkills,
		svcStatus:        DemoSystemStatus,
		trustBundles:     DemoTrustBundles,
		apiKeys:          DemoAPIKeys,
		securityEvents:   DemoSecurityEvents,
		agentsTable:      agentT,
		policiesTable:    policyT,
		auditTable:       auditT,
		hitlTable:        hitlT,
		airportTable:     airportT,
		skillsTable:      skillsT,
		statusTable:      statusT,
		federationTable:  federationT,
		apiKeysTable:     apiKeysT,
		securityTable:    securityT,
		onboardStep:      0,
	}
}

// Init is called first by Bubble Tea
func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		nextBootStepCmd(0),
	)
}

// ---------------------------------------------------------------------------
// TABLE BUILDERS
// ---------------------------------------------------------------------------

func createAgentsTable(agents []Agent) table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 12},
		{Title: "DID", Width: 20},
		{Title: "Trust", Width: 6},
		{Title: "Status", Width: 11},
	}
	rows := make([]table.Row, len(agents))
	for i, a := range agents {
		rows[i] = table.Row{a.Name, a.DID, fmt.Sprintf("%d%%", a.Trust), a.Status}
	}
	return styledTable(columns, rows, 6)
}

func createPoliciesTable(policies []Policy) table.Model {
	columns := []table.Column{
		{Title: "Action", Width: 16},
		{Title: "Decision", Width: 14},
		{Title: "Reason", Width: 20},
	}
	rows := make([]table.Row, len(policies))
	for i, p := range policies {
		rows[i] = table.Row{p.Action, p.Decision, p.Reason}
	}
	return styledTable(columns, rows, 6)
}

func createAuditTable(auditLogs []AuditLog) table.Model {
	columns := []table.Column{
		{Title: "Time", Width: 8},
		{Title: "Agent", Width: 11},
		{Title: "Event", Width: 31},
	}
	rows := make([]table.Row, len(auditLogs))
	for i, a := range auditLogs {
		rows[i] = table.Row{a.Timestamp, a.Agent, a.Event}
	}
	return styledTable(columns, rows, 6)
}

func createHITLTable(reqs []HITLRequest) table.Model {
	columns := []table.Column{
		{Title: "ID", Width: 9},
		{Title: "Agent", Width: 10},
		{Title: "Action", Width: 12},
		{Title: "Risk", Width: 7},
		{Title: "Status", Width: 8},
		{Title: "Time", Width: 7},
	}
	rows := make([]table.Row, len(reqs))
	for i, r := range reqs {
		rows[i] = table.Row{r.ID, r.AgentID, r.Action, r.RiskLevel, r.Status, r.CreatedAt}
	}
	return styledTable(columns, rows, 6)
}

func createAirportTable(agents []AirportAgent) table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 11},
		{Title: "Status", Width: 8},
		{Title: "Trust", Width: 6},
		{Title: "Tags", Width: 16},
		{Title: "ID", Width: 10},
	}
	rows := make([]table.Row, len(agents))
	for i, a := range agents {
		rows[i] = table.Row{
			a.Name,
			strings.ToUpper(a.Status),
			fmt.Sprintf("%.0f%%", a.TrustScore*100),
			a.Tags,
			a.AgentID,
		}
	}
	return styledTable(columns, rows, 6)
}

func createSkillsTable(skills []Skill) table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 12},
		{Title: "Category", Width: 12},
		{Title: "Risk", Width: 8},
		{Title: "TrustMin", Width: 8},
		{Title: "Lvl", Width: 4},
	}
	rows := make([]table.Row, len(skills))
	for i, sk := range skills {
		rows[i] = table.Row{sk.Name, sk.Category, sk.RiskLevel, sk.TrustMin, sk.Proficiency}
	}
	return styledTable(columns, rows, 6)
}

func createStatusTable(svcs []ServiceStatus) table.Model {
	columns := []table.Column{
		{Title: "Service", Width: 16},
		{Title: "Status", Width: 6},
		{Title: "Uptime", Width: 9},
		{Title: "Note", Width: 19},
	}
	rows := make([]table.Row, len(svcs))
	for i, s := range svcs {
		rows[i] = table.Row{s.Name, s.Status, s.Uptime, s.Note}
	}
	return styledTable(columns, rows, 6)
}

func styledTable(columns []table.Column, rows []table.Row, height int) table.Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)
	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedRowStyle
	t.SetStyles(s)
	return t
}

func createFederationTable(bundles []TrustBundle) table.Model {
	columns := []table.Column{
		{Title: "Trust Domain", Width: 13},
		{Title: "Type", Width: 11},
		{Title: "Source", Width: 7},
		{Title: "Fed", Width: 4},
		{Title: "Status", Width: 9},
	}
	rows := make([]table.Row, len(bundles))
	for i, b := range bundles {
		rows[i] = table.Row{b.TrustDomain, b.Type, b.Source, b.Federated, b.Status}
	}
	return styledTable(columns, rows, 6)
}

func createAPIKeysTable(keys []APIKeyEntry) table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 12},
		{Title: "ID", Width: 9},
		{Title: "Tenant", Width: 10},
		{Title: "Status", Width: 8},
		{Title: "Created", Width: 11},
	}
	rows := make([]table.Row, len(keys))
	for i, k := range keys {
		rows[i] = table.Row{k.Name, k.ID, k.TenantID, k.Status, k.CreatedAt}
	}
	return styledTable(columns, rows, 6)
}

func createSecurityTable(events []SecurityEvent) table.Model {
	columns := []table.Column{
		{Title: "Workflow", Width: 20},
		{Title: "Result", Width: 9},
		{Title: "Branch", Width: 8},
		{Title: "Time", Width: 6},
	}
	rows := make([]table.Row, len(events))
	for i, e := range events {
		conclusion := strings.ToUpper(e.Conclusion)
		rows[i] = table.Row{e.Workflow, conclusion, e.Branch, e.RunAt}
	}
	return styledTable(columns, rows, 6)
}

// ---------------------------------------------------------------------------
// LIVE FETCH COMMANDS
// ---------------------------------------------------------------------------


type fetchAgentsMsg struct {
	agents []Agent
	err    error
}

func fetchAgentsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListAgents()
		if err != nil {
			return fetchAgentsMsg{err: err}
		}
		agentsList, ok := res["agents"].([]interface{})
		if !ok {
			return fetchAgentsMsg{agents: []Agent{}}
		}
		var agents []Agent
		for _, item := range agentsList {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			agentID, _ := m["agent_id"].(string)
			status, _ := m["status"].(string)
			trust := 0
			if tVal, ok := m["trust_score"].(float64); ok {
				trust = int(tVal * 100)
			}
			did := "did:eyevesa:agent:FFFF"
			if len(agentID) >= 4 {
				did = "did:eyevesa:agent:" + strings.ToUpper(agentID[:4])
			}
			agents = append(agents, Agent{
				Name:   name,
				DID:    did,
				Trust:  trust,
				Status: strings.ToUpper(status),
			})
		}
		return fetchAgentsMsg{agents: agents}
	}
}

type fetchPoliciesMsg struct {
	policies []Policy
	err      error
}

func fetchPoliciesCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListResources()
		if err != nil {
			return fetchPoliciesMsg{err: err}
		}
		resourcesList, ok := res["resources"].([]interface{})
		if !ok {
			return fetchPoliciesMsg{policies: []Policy{}}
		}
		var policies []Policy
		policies = append(policies, Policy{Action: "Global: allowed_tools", Decision: "ALLOW", Reason: "Tool in allowed list"})
		policies = append(policies, Policy{Action: "Global: cost_over_budget", Decision: "DENY", Reason: "Cost exceeds threshold"})
		for _, item := range resourcesList {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			risk, _ := m["risk_level"].(string)
			decision := "ALLOW"
			reason := "Low risk resource"
			if strings.ToLower(risk) == "high" {
				decision = "HUMAN_APPROVAL"
				reason = "High risk production"
			} else if strings.ToLower(risk) == "critical" {
				decision = "DENY"
				reason = "Critical resource"
			}
			policies = append(policies, Policy{Action: name, Decision: decision, Reason: reason})
		}
		return fetchPoliciesMsg{policies: policies}
	}
}

type fetchAuditMsg struct {
	auditLogs []AuditLog
	err       error
}

func fetchAuditCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.Audit("", 50, 0)
		if err != nil {
			return fetchAuditMsg{err: err}
		}
		logsList, ok := res["audit_logs"].([]interface{})
		if !ok {
			logsList, ok = res["logs"].([]interface{})
		}
		if !ok {
			return fetchAuditMsg{auditLogs: []AuditLog{}}
		}
		var auditLogs []AuditLog
		for _, item := range logsList {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			timestamp, _ := m["timestamp"].(string)
			if timestamp == "" {
				timestamp, _ = m["created_at"].(string)
			}
			if len(timestamp) > 19 {
				timestamp = timestamp[11:19]
			} else if len(timestamp) > 8 {
				timestamp = timestamp[:8]
			}
			agent, _ := m["agent_name"].(string)
			if agent == "" {
				agent, _ = m["agent_id"].(string)
				if len(agent) > 8 {
					agent = agent[:8]
				}
			}
			action, _ := m["action"].(string)
			decision, _ := m["decision"].(string)
			hash, _ := m["audit_hash"].(string)
			if hash == "" {
				hash, _ = m["hash"].(string)
			}
			event := fmt.Sprintf("requested %s (decision: %s)", action, decision)
			if hash != "" && len(hash) > 10 {
				event += fmt.Sprintf(" | %s", hash[:10])
			}
			auditLogs = append(auditLogs, AuditLog{
				Timestamp: timestamp,
				Agent:     agent,
				Event:     event,
				Decision:  decision,
				Hash:      hash,
			})
		}
		return fetchAuditMsg{auditLogs: auditLogs}
	}
}

type fetchHITLMsg struct {
	requests []HITLRequest
	err      error
}

func fetchHITLCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListHILTPending()
		if err != nil {
			return fetchHITLMsg{err: err}
		}
		list, ok := res["requests"].([]interface{})
		if !ok {
			list, _ = res["approvals"].([]interface{})
		}
		var reqs []HITLRequest
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			if id == "" {
				id, _ = m["approval_id"].(string)
			}
			agentID, _ := m["agent_id"].(string)
			action, _ := m["action"].(string)
			resourceID, _ := m["resource_id"].(string)
			riskLevel, _ := m["risk_level"].(string)
			status, _ := m["status"].(string)
			createdAt, _ := m["created_at"].(string)
			if len(createdAt) > 19 {
				createdAt = createdAt[11:19]
			}
			reqs = append(reqs, HITLRequest{
				ID: id, AgentID: agentID, Action: action,
				ResourceID: resourceID, RiskLevel: strings.ToUpper(riskLevel),
				Status: strings.ToUpper(status), CreatedAt: createdAt,
			})
		}
		return fetchHITLMsg{requests: reqs}
	}
}

type fetchAirportMsg struct {
	agents []AirportAgent
	err    error
}

func fetchAirportCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.AirportListOnline()
		if err != nil {
			return fetchAirportMsg{err: err}
		}
		list, ok := res["agents"].([]interface{})
		if !ok {
			return fetchAirportMsg{agents: []AirportAgent{}}
		}
		var agents []AirportAgent
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			agentID, _ := m["agent_id"].(string)
			name, _ := m["name"].(string)
			status, _ := m["status"].(string)
			trust, _ := m["trust_score"].(float64)
			var tags []string
			if t, ok := m["tags"].([]interface{}); ok {
				for _, tag := range t {
					if s, ok := tag.(string); ok {
						tags = append(tags, s)
					}
				}
			}
			agents = append(agents, AirportAgent{
				AgentID: agentID, Name: name, Status: status,
				TrustScore: trust, Tags: strings.Join(tags, ","),
			})
		}
		return fetchAirportMsg{agents: agents}
	}
}

type fetchSkillsMsg struct {
	skills []Skill
	err    error
}

func fetchSkillsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListSkills("")
		if err != nil {
			return fetchSkillsMsg{err: err}
		}
		list, ok := res["skills"].([]interface{})
		if !ok {
			return fetchSkillsMsg{skills: []Skill{}}
		}
		var skills []Skill
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			name, _ := m["name"].(string)
			cat, _ := m["category"].(string)
			risk, _ := m["risk_level"].(string)
			trust := "0.00"
			if tv, ok := m["required_trust_min"].(float64); ok {
				trust = fmt.Sprintf("%.2f", tv)
			}
			prof := "1"
			if pv, ok := m["required_proficiency"].(float64); ok {
				prof = fmt.Sprintf("%d", int(pv))
			}
			skills = append(skills, Skill{
				ID: id, Name: name, Category: cat,
				RiskLevel: risk, TrustMin: trust, Proficiency: prof,
			})
		}
		return fetchSkillsMsg{skills: skills}
	}
}

type fetchStatusMsg struct {
	services []ServiceStatus
	err      error
}

func fetchStatusCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		_, err := client.Health()
		status := "UP"
		if err != nil {
			status = "DOWN"
		}
		return fetchStatusMsg{
			services: []ServiceStatus{
				{Name: "gateway-control", Status: status, Uptime: "--", Note: "HTTP+gRPC API"},
				{Name: "gateway-core", Status: status, Uptime: "--", Note: "Rust MCP proxy"},
				{Name: "postgres", Status: status, Uptime: "--", Note: "pgvector enabled"},
				{Name: "opa-engine", Status: status, Uptime: "--", Note: "Rego authz"},
				{Name: "audit-ledger", Status: status, Uptime: "--", Note: "Ed25519 signed"},
			},
		}
	}
}

// registerAgentCmd fires an agent registration API call
type registerAgentMsg struct {
	result string
	err    error
}

func registerAgentCmd(client *api.Client, name, owner, caps string) tea.Cmd {
	return func() tea.Msg {
		capabilities := strings.Split(caps, ",")
		for i := range capabilities {
			capabilities[i] = strings.TrimSpace(capabilities[i])
		}
		res, err := client.RegisterAgent(name, owner, capabilities, []string{"*"}, 100.0, "allow", []string{})
		if err != nil {
			return registerAgentMsg{err: err}
		}
		agentID, _ := res["agent_id"].(string)
		return registerAgentMsg{result: fmt.Sprintf("Registered! agent_id: %s", agentID)}
	}
}

// ── Federation Fetch ─────────────────────────────────────────────────────────

type fetchFederationMsg struct {
	bundles []TrustBundle
	err     error
}

func fetchFederationCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListTrustBundles(false)
		if err != nil {
			return fetchFederationMsg{err: err}
		}
		list, ok := res["bundles"].([]interface{})
		if !ok {
			return fetchFederationMsg{bundles: []TrustBundle{}}
		}
		var bundles []TrustBundle
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			td, _ := m["trust_domain"].(string)
			typ, _ := m["bundle_type"].(string)
			src, _ := m["source"].(string)
			fed := "NO"
			if f, ok := m["is_federated"].(bool); ok && f {
				fed = "YES"
			}
			status := "ACTIVE"
			if s, ok := m["status"].(string); ok && s != "" {
				status = strings.ToUpper(s)
			}
			bundles = append(bundles, TrustBundle{
				TrustDomain: td, Type: typ, Source: src,
				Federated: fed, Status: status,
			})
		}
		return fetchFederationMsg{bundles: bundles}
	}
}

// ── API Keys Fetch ────────────────────────────────────────────────────────────

type fetchAPIKeysMsg struct {
	keys []APIKeyEntry
	err  error
}

func fetchAPIKeysCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		res, err := client.ListAPIKeys()
		if err != nil {
			return fetchAPIKeysMsg{err: err}
		}
		list, ok := res["api_keys"].([]interface{})
		if !ok {
			list, _ = res["keys"].([]interface{})
		}
		var keys []APIKeyEntry
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := m["id"].(string)
			name, _ := m["name"].(string)
			tenant, _ := m["tenant_id"].(string)
			status, _ := m["status"].(string)
			if status == "" {
				status = "ACTIVE"
			}
			createdAt, _ := m["created_at"].(string)
			if len(createdAt) > 10 {
				createdAt = createdAt[:10]
			}
			keys = append(keys, APIKeyEntry{
				ID: id, Name: name, TenantID: tenant,
				Status: strings.ToUpper(status), CreatedAt: createdAt,
			})
		}
		return fetchAPIKeysMsg{keys: keys}
	}
}

// ── Security Fetch ─────────────────────────────────────────────────────────────

type fetchSecurityMsg struct {
	events []SecurityEvent
	err    error
}

func fetchSecurityCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		runs, err := client.SecurityWorkflowRuns()
		if err != nil {
			return fetchSecurityMsg{err: err}
		}
		var events []SecurityEvent
		for _, run := range runs {
			name, _ := run["name"].(string)
			status, _ := run["status"].(string)
			conclusion, _ := run["conclusion"].(string)
			if conclusion == "" {
				conclusion = "in_progress"
			}
			branch := ""
			if hc, ok := run["head_commit"].(map[string]interface{}); ok {
				branch, _ = hc["branch"].(string)
			}
			if branch == "" {
				if hb, ok := run["head_branch"].(string); ok {
					branch = hb
				}
			}
			runAt := ""
			if ra, ok := run["created_at"].(string); ok && len(ra) >= 16 {
				runAt = ra[11:16]
			}
			events = append(events, SecurityEvent{
				Workflow:   name,
				Status:     status,
				Conclusion: conclusion,
				Branch:     branch,
				RunAt:      runAt,
			})
		}
		return fetchSecurityMsg{events: events}
	}
}

