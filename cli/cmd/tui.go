package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hafizaljohari/eyeVesa/cli/internal/api"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long: `Launch an interactive terminal dashboard for eyeVesa.

Navigate using:
  - Tab/Shift+Tab: Switch between views
  - ↑/↓: Navigate list items
  - Enter: View details
  - n: Register new agent (in Agents view)
  - r: Refresh current view
  - a: Approve HITL request (in HITL view)
  - d: Deny HITL request (in HITL view)
  - q/Ctrl+C: Quit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		p := tea.NewProgram(initialModel(client), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	addStartCommand(tuiCmd)
}

type view int

const (
	viewDashboard view = iota
	viewAgents
	viewResources
	viewHITL
	viewAudit
	viewFederation
	viewAPIKeys
	viewSecurity
	viewRegisterAgent
	viewCreateAPIKey
)

type registerStep int

const (
	stepName registerStep = iota
	stepOwner
	stepCapabilities
	stepAllowedTools
	stepBudget
	stepSubmit
)

type focusInputMsg struct{}

type model struct {
	client           *api.Client
	currentView      view
	agents           []map[string]interface{}
	resources        []map[string]interface{}
	hitlPending      []map[string]interface{}
	auditLogs        []map[string]interface{}
	apiKeys          []map[string]interface{}
	federationHealth map[string]interface{}
	federationPeers  []map[string]interface{}
	federationOnline []map[string]interface{}
	securityRuns     []map[string]interface{}
	err              error
	loading          bool
	spinner          spinner.Model
	table            table.Model
	ready            bool
	selectedIdx      int
	statusMsg        string
	width            int
	height           int

	formStep     registerStep
	formInputs   []textinput.Model
	formIdx      int
	registerOk   bool
	inputFocused bool

	apiKeyInputs []textinput.Model
	apiKeyStep   int
}

type tickMsg struct{}
type refreshMsg struct{}
type agentsLoadedMsg struct {
	agents []map[string]interface{}
	err    error
}
type resourcesLoadedMsg struct {
	resources []map[string]interface{}
	err       error
}
type hitlLoadedMsg struct {
	pending []map[string]interface{}
	err     error
}
type auditLoadedMsg struct {
	logs []map[string]interface{}
	err  error
}
type federationLoadedMsg struct {
	health map[string]interface{}
	peers  []map[string]interface{}
	online []map[string]interface{}
	err    error
}

type apiKeysLoadedMsg struct {
	keys []map[string]interface{}
	err  error
}
type securityLoadedMsg struct {
	runs []map[string]interface{}
	err  error
}

type apiKeyCreatedMsg struct {
	name string
}

type apiKeyCreateErrMsg struct {
	err error
}

type agentRegisteredMsg struct {
	id   string
	name string
}
type registerErrMsg struct {
	err error
}

type hitlApprovedMsg struct {
	id string
}
type hitlDeniedMsg struct {
	id string
}
type hitlActionErrMsg struct {
	err error
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			Padding(0, 1).
			SetString("eyeVesa")

	viewStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6B7280")).
			Padding(0, 1).
			Margin(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Padding(1, 0)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true)
)

func newFormInputs() []textinput.Model {
	inputs := make([]textinput.Model, 5)
	var t textinput.Model

	t = textinput.New()
	t.Placeholder = "e.g. hermes-ops"
	t.CharLimit = 64
	t.Width = 40
	inputs[0] = t

	t = textinput.New()
	t.Placeholder = "e.g. org:devops"
	t.CharLimit = 64
	t.Width = 40
	inputs[1] = t

	t = textinput.New()
	t.Placeholder = "e.g. read,write,deploy"
	t.CharLimit = 128
	t.Width = 40
	inputs[2] = t

	t = textinput.New()
	t.Placeholder = "e.g. k8s_deploy,log_search"
	t.CharLimit = 128
	t.Width = 40
	inputs[3] = t

	t = textinput.New()
	t.Placeholder = "e.g. 500"
	t.CharLimit = 10
	t.Width = 40
	inputs[4] = t

	return inputs
}

func initialModel(client *api.Client) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	t := table.New()
	t.SetColumns([]table.Column{
		{Title: "ID", Width: 36},
		{Title: "Name", Width: 20},
		{Title: "Owner", Width: 15},
		{Title: "Status", Width: 10},
		{Title: "Trust", Width: 8},
	})

	inputs := newFormInputs()

	akInputs := make([]textinput.Model, 2)
	akInputs[0] = textinput.New()
	akInputs[0].Placeholder = "e.g. my-agent-key"
	akInputs[0].CharLimit = 64
	akInputs[0].Width = 40

	akInputs[1] = textinput.New()
	akInputs[1].Placeholder = "e.g. org:phos (optional)"
	akInputs[1].CharLimit = 64
	akInputs[1].Width = 40

	return model{
		client:       client,
		currentView:  viewDashboard,
		spinner:      s,
		loading:      true,
		table:        t,
		formInputs:   inputs,
		apiKeyInputs: akInputs,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadAllData,
	)
}

func (m model) loadAllData() tea.Msg {
	return refreshMsg{}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.currentView == viewRegisterAgent {
			return m.handleRegisterKey(msg)
		}
		if m.currentView == viewCreateAPIKey {
			return m.handleCreateAPIKeyKey(msg)
		}
		newM, cmd := m.handleMainKey(msg)
		return newM, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 10)
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case focusInputMsg:
		if m.currentView == viewRegisterAgent && !m.inputFocused {
			m.inputFocused = true
			return m, m.formInputs[m.formIdx].Focus()
		}
		if m.currentView == viewCreateAPIKey && !m.inputFocused {
			m.inputFocused = true
			return m, m.apiKeyInputs[0].Focus()
		}
		return m, nil

	case apiKeysLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.apiKeys = msg.keys
		}

	case apiKeyCreatedMsg:
		m.statusMsg = fmt.Sprintf("API key created: %s", msg.name)
		m.currentView = viewAPIKeys
		m.loading = true
		return m, m.refreshCurrentView

	case apiKeyCreateErrMsg:
		m.err = msg.err
		m.currentView = viewAPIKeys

	case refreshMsg:
		return m, tea.Batch(
			m.loadAgents,
			m.loadResources,
			m.loadHITL,
			m.loadAudit,
			m.loadFederation,
			m.loadAPIKeys,
			m.loadSecurity,
		)

	case agentsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.agents = msg.agents
		}

	case resourcesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.resources = msg.resources
		}

	case hitlLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.hitlPending = msg.pending
		}

	case auditLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.auditLogs = msg.logs
		}

	case federationLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.federationHealth = msg.health
			m.federationPeers = msg.peers
			m.federationOnline = msg.online
		}

	case agentRegisteredMsg:
		m.statusMsg = fmt.Sprintf("Agent registered: %s", msg.name)
		m.currentView = viewAgents
		m.loading = true
		return m, m.refreshCurrentView

	case registerErrMsg:
		m.err = msg.err
		m.currentView = viewAgents

	case hitlApprovedMsg:
		m.loading = false
		m.statusMsg = fmt.Sprintf("Approved %s", msg.id)
		m.selectedIdx = 0
		return m, m.loadHITL

	case hitlDeniedMsg:
		m.loading = false
		m.statusMsg = fmt.Sprintf("Denied %s", msg.id)
		m.selectedIdx = 0
		return m, m.loadHITL

	case hitlActionErrMsg:
		m.loading = false
		m.err = msg.err

	case securityLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.securityRuns = msg.runs
		}
	}

	return m, nil
}

func (m model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.loading = true
		return m, nil

	case "tab":
		m.currentView = (m.currentView + 1) % 9
		m.formIdx = 0
		m.formStep = stepName
		m.formInputs = newFormInputs()
		m.apiKeyStep = 0
		m.apiKeyInputs = []textinput.Model{
			func() textinput.Model {
				t := textinput.New()
				t.Placeholder = "e.g. my-agent-key"
				t.CharLimit = 64
				t.Width = 40
				return t
			}(),
			func() textinput.Model {
				t := textinput.New()
				t.Placeholder = "e.g. org:phos (optional)"
				t.CharLimit = 64
				t.Width = 40
				return t
			}(),
		}
		m.selectedIdx = 0
		m.statusMsg = ""
		m.err = nil
		m.registerOk = false
		m.inputFocused = false
		return m, nil

	case "shift+tab":
		if m.currentView == 0 {
			m.currentView = 8
		} else {
			m.currentView--
		}
		m.formIdx = 0
		m.formStep = stepName
		m.formInputs = newFormInputs()
		m.apiKeyStep = 0
		m.apiKeyInputs = []textinput.Model{
			func() textinput.Model {
				t := textinput.New()
				t.Placeholder = "e.g. my-agent-key"
				t.CharLimit = 64
				t.Width = 40
				return t
			}(),
			func() textinput.Model {
				t := textinput.New()
				t.Placeholder = "e.g. org:phos (optional)"
				t.CharLimit = 64
				t.Width = 40
				return t
			}(),
		}
		m.selectedIdx = 0
		m.statusMsg = ""
		m.err = nil
		m.registerOk = false
		m.inputFocused = false
		return m, nil

	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case "down", "j":
		m.selectedIdx++

	case "r":
		m.loading = true
		m.statusMsg = ""
		m.err = nil

	case "n":
		if m.currentView == viewAgents {
			m.currentView = viewRegisterAgent
			m.formStep = stepName
			m.formIdx = 0
			m.formInputs = newFormInputs()
			m.err = nil
			m.registerOk = false
			m.inputFocused = false
			return m, focusInputCmd
		}

	case "c":
		if m.currentView == viewAPIKeys {
			m.currentView = viewCreateAPIKey
			m.apiKeyStep = 0
			m.apiKeyInputs = []textinput.Model{
				func() textinput.Model {
					t := textinput.New()
					t.Placeholder = "e.g. my-agent-key"
					t.CharLimit = 64
					t.Width = 40
					return t
				}(),
				func() textinput.Model {
					t := textinput.New()
					t.Placeholder = "e.g. org:phos (optional)"
					t.CharLimit = 64
					t.Width = 40
					return t
				}(),
			}
			m.err = nil
			m.inputFocused = false
			return m, focusInputCmd
		}

	case "a":
		if m.currentView == viewHITL && len(m.hitlPending) > 0 && m.selectedIdx < len(m.hitlPending) {
			approvalID, _ := m.hitlPending[m.selectedIdx]["id"].(string)
			if approvalID == "" {
				approvalID, _ = m.hitlPending[m.selectedIdx]["approval_id"].(string)
			}
			if approvalID != "" {
				m.loading = true
				m.statusMsg = "Approving..."
				m.err = nil
				return m, func() tea.Msg {
					_, err := m.client.ApproveHILT(approvalID, "tui")
					if err != nil {
						return hitlActionErrMsg{err: err}
					}
					return hitlApprovedMsg{id: approvalID}
				}
			}
		}

	case "d":
		if m.currentView == viewHITL && len(m.hitlPending) > 0 && m.selectedIdx < len(m.hitlPending) {
			approvalID, _ := m.hitlPending[m.selectedIdx]["id"].(string)
			if approvalID == "" {
				approvalID, _ = m.hitlPending[m.selectedIdx]["approval_id"].(string)
			}
			if approvalID != "" {
				m.loading = true
				m.statusMsg = "Denying..."
				m.err = nil
				return m, func() tea.Msg {
					_, err := m.client.DenyHILT(approvalID, "tui")
					if err != nil {
						return hitlActionErrMsg{err: err}
					}
					return hitlDeniedMsg{id: approvalID}
				}
			}
		}
	}
	return m, nil
}

func focusInputCmd() tea.Msg {
	return focusInputMsg{}
}

func (m model) handleRegisterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentView = viewAgents
		m.formStep = stepName
		m.formIdx = 0
		for i := range m.formInputs {
			m.formInputs[i].Blur()
		}
		m.formInputs = newFormInputs()
		m.err = nil
		m.inputFocused = false
		return m, nil

	case "tab", "enter":
		if m.formStep == stepSubmit {
			return m, m.submitAgent()
		}

		m.formInputs[m.formIdx].Blur()
		if m.formStep < stepSubmit {
			m.formStep++
			m.formIdx++
		}
		if m.formStep == stepSubmit {
			m.inputFocused = false
			return m, nil
		}
		m.inputFocused = true
		return m, m.formInputs[m.formIdx].Focus()
	}

	var cmd tea.Cmd
	m.formInputs[m.formIdx], cmd = m.formInputs[m.formIdx].Update(msg)
	return m, cmd
}

func (m model) submitAgent() tea.Cmd {
	name := strings.TrimSpace(m.formInputs[0].Value())
	owner := strings.TrimSpace(m.formInputs[1].Value())
	caps := parseList(m.formInputs[2].Value())
	tools := parseList(m.formInputs[3].Value())
	budget := 0.0
	if bv := strings.TrimSpace(m.formInputs[4].Value()); bv != "" {
		fmt.Sscanf(bv, "%f", &budget)
	}

	if name == "" || owner == "" {
		return func() tea.Msg {
			return registerErrMsg{err: fmt.Errorf("name and owner are required")}
		}
	}

	return func() tea.Msg {
		result, err := m.client.RegisterAgent(name, owner, caps, tools, budget, "no_chain", []string{})
		if err != nil {
			return registerErrMsg{err: err}
		}
		id, _ := result["agent_id"].(string)
		return agentRegisteredMsg{id: id, name: name}
	}
}

func parseList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func truncateID(id string, maxLen int) string {
	if len(id) > maxLen {
		return id[:maxLen] + "..."
	}
	return id
}

func toMapSlice(raw interface{}) []map[string]interface{} {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func (m model) loadAgents() tea.Msg {
	result, err := m.client.ListAgents()
	if err != nil {
		return agentsLoadedMsg{err: err}
	}
	agents := toMapSlice(result["agents"])
	return agentsLoadedMsg{agents: agents}
}

func (m model) loadResources() tea.Msg {
	result, err := m.client.ListResources()
	if err != nil {
		return resourcesLoadedMsg{err: err}
	}
	resources := toMapSlice(result["resources"])
	return resourcesLoadedMsg{resources: resources}
}

func (m model) loadHITL() tea.Msg {
	result, err := m.client.ListHILTPending()
	if err != nil {
		return hitlLoadedMsg{err: err}
	}
	pending := toMapSlice(result["approvals"])
	return hitlLoadedMsg{pending: pending}
}

func (m model) loadAudit() tea.Msg {
	if len(m.agents) == 0 {
		return auditLoadedMsg{}
	}
	agentID, ok := m.agents[0]["agent_id"].(string)
	if !ok {
		return auditLoadedMsg{}
	}
	result, err := m.client.Audit(agentID, 20, 0)
	if err != nil {
		return auditLoadedMsg{err: err}
	}
	logs := toMapSlice(result["entries"])
	return auditLoadedMsg{logs: logs}
}

func (m model) refreshCurrentView() tea.Msg {
	return refreshMsg{}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Loading..."
	}

	var b strings.Builder

	b.WriteString(titleStyle.String())
	b.WriteString("\n\n")

	views := []string{"Dashboard", "Agents", "Resources", "HITL", "Audit", "Federation", "API Keys", "Security"}
	displayView := m.currentView
	if m.currentView == viewRegisterAgent {
		displayView = viewAgents
	}
	if m.currentView == viewCreateAPIKey {
		displayView = viewAPIKeys
	}
	for i, v := range views {
		if i == int(displayView) {
			b.WriteString(selectedStyle.Render("▶ " + v))
		} else {
			b.WriteString(viewStyle.Render("  " + v))
		}
		b.WriteString("  ")
	}
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("❌ Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	switch m.currentView {
	case viewDashboard:
		b.WriteString(m.renderDashboard())
	case viewAgents:
		b.WriteString(m.renderAgents())
	case viewResources:
		b.WriteString(m.renderResources())
	case viewHITL:
		b.WriteString(m.renderHITL())
	case viewAudit:
		b.WriteString(m.renderAudit())
	case viewFederation:
		b.WriteString(m.renderFederation())
	case viewAPIKeys:
		b.WriteString(m.renderAPIKeys())
	case viewSecurity:
		b.WriteString(m.renderSecurity())
	case viewRegisterAgent:
		b.WriteString(m.renderRegisterForm())
	case viewCreateAPIKey:
		b.WriteString(m.renderCreateAPIKeyForm())
	}

	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ " + m.statusMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Tab: switch view | ↑↓: navigate | n: register agent | c: create API key | r: refresh | q: quit"))

	return b.String()
}

func (m model) loadFederation() tea.Msg {
	health, err := m.client.FederationHealth()
	if err != nil {
		return federationLoadedMsg{err: err}
	}

	peersResult, err := m.client.FederationListPeers()
	if err != nil {
		return federationLoadedMsg{err: err}
	}

	onlineResult, err := m.client.FederationListOnline()
	if err != nil {
		return federationLoadedMsg{err: err}
	}

	return federationLoadedMsg{
		health: health,
		peers:  toMapSlice(peersResult["peers"]),
		online: toMapSlice(onlineResult["agents"]),
	}
}

func (m model) loadAPIKeys() tea.Msg {
	result, err := m.client.ListAPIKeys()
	if err != nil {
		return apiKeysLoadedMsg{err: err}
	}
	keys := toMapSlice(result["keys"])
	return apiKeysLoadedMsg{keys: keys}
}

func (m model) renderFederation() string {
	var b strings.Builder

	if m.loading && m.federationHealth == nil {
		return m.spinner.View() + " Loading federation data..."
	}

	b.WriteString(boxStyle.Render("Federation Health"))
	b.WriteString("\n")

	if m.federationHealth != nil {
		status, _ := m.federationHealth["status"].(string)
		activeGateways, _ := m.federationHealth["active_gateways"].(float64)
		onlineAgents, _ := m.federationHealth["online_federated_agents"].(float64)

		if status == "healthy" {
			b.WriteString(successStyle.Render("  ✓ Status: " + status))
		} else {
			b.WriteString(errorStyle.Render("  ✗ Status: " + status))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  Active Gateways: %.0f\n", activeGateways))
		b.WriteString(fmt.Sprintf("  Online Federated Agents: %.0f\n", onlineAgents))
	} else {
		b.WriteString(errorStyle.Render("  ✗ Federation: DISCONNECTED"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(boxStyle.Render(fmt.Sprintf("Federation Peers (%d)", len(m.federationPeers))))
	b.WriteString("\n\n")

	if len(m.federationPeers) == 0 {
		b.WriteString("No federation peers registered\n")
	} else {
		for i, peer := range m.federationPeers {
			cursor := "  "
			if i == m.selectedIdx {
				cursor = "▶ "
			}

			name, _ := peer["name"].(string)
			peerStatus, _ := peer["status"].(string)
			peerType, _ := peer["peer_type"].(string)
			trustScore, _ := peer["trust_score"].(float64)
			agentCount, _ := peer["agent_count"].(float64)

			statusIndicator := successStyle.Render("●")
			if peerStatus != "active" {
				statusIndicator = errorStyle.Render("●")
			}

			line := fmt.Sprintf("%s%s %-20s %-10s %-8s trust: %.2f agents: %.0f",
				cursor, statusIndicator, name, peerType, peerStatus, trustScore, agentCount)
			if i == m.selectedIdx {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(boxStyle.Render(fmt.Sprintf("Online Federated Agents (%d)", len(m.federationOnline))))
	b.WriteString("\n\n")

	if len(m.federationOnline) == 0 {
		b.WriteString("No online federated agents\n")
	} else {
		for i, agent := range m.federationOnline {
			cursor := "  "
			if len(m.federationPeers)+i == m.selectedIdx {
				cursor = "▶ "
			}

			name, _ := agent["name"].(string)
			owner, _ := agent["owner"].(string)
			agentStatus, _ := agent["heartbeat_status"].(string)
			trust, _ := agent["trust_score"].(float64)
			scope, _ := agent["scope"].(string)

			line := fmt.Sprintf("%s%-25s %-15s %-8s trust: %.2f scope: %s",
				cursor, name, owner, agentStatus, trust, scope)
			b.WriteString(selectedStyle.Render(line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m model) renderAPIKeys() string {
	var b strings.Builder

	if m.loading && len(m.apiKeys) == 0 {
		return m.spinner.View() + " Loading API keys..."
	}

	b.WriteString(boxStyle.Render(fmt.Sprintf("API Keys (%d)", len(m.apiKeys))))
	b.WriteString("\n\n")

	if len(m.apiKeys) == 0 {
		b.WriteString("No API keys\n\n")
		b.WriteString(helpStyle.Render("  Press c to create a new API key"))
		return b.String()
	}

	for i, key := range m.apiKeys {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "▶ "
		}

		name, _ := key["name"].(string)
		keyID, _ := key["key_id"].(string)
		isActive, _ := key["is_active"].(bool)
		created, _ := key["created_at"].(string)
		tenantID, _ := key["tenant_id"].(string)

		statusIcon := successStyle.Render("●")
		statusText := "active"
		if !isActive {
			statusIcon = errorStyle.Render("●")
			statusText = "revoked"
		}

		shortID := keyID
		if len(keyID) > 8 {
			shortID = keyID[:8] + "..."
		}

		line := fmt.Sprintf("%s%s %-20s %-12s %-8s",
			cursor, statusIcon, name, shortID, statusText)
		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")

		if i == m.selectedIdx {
			b.WriteString(viewStyle.Render(fmt.Sprintf("  ID: %s", keyID)))
			b.WriteString("\n")
			if tenantID != "" {
				b.WriteString(viewStyle.Render(fmt.Sprintf("  Tenant: %s", tenantID)))
				b.WriteString("\n")
			}
			b.WriteString(viewStyle.Render(fmt.Sprintf("  Created: %s", created)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("c: create API key"))

	return b.String()
}

func (m model) loadSecurity() tea.Msg {
	runs, err := m.client.SecurityWorkflowRuns()
	if err != nil {
		return securityLoadedMsg{err: err}
	}
	return securityLoadedMsg{runs: runs}
}

func (m model) renderSecurity() string {
	var b strings.Builder
	if m.loading && len(m.securityRuns) == 0 {
		return m.spinner.View() + " Loading security harness status..."
	}

	b.WriteString(boxStyle.Render("Security Harness Status (GitHub Actions)"))
	b.WriteString("\n\n")

	if len(m.securityRuns) == 0 {
		b.WriteString("No security workflow runs found.\n")
		b.WriteString(helpStyle.Render("Set GITHUB_TOKEN and optional EYEVESA_GITHUB_REPO=owner/repo, then press r"))
		return b.String()
	}

	for _, run := range m.securityRuns {
		name, _ := run["name"].(string)
		status, _ := run["status"].(string)
		conclusion, _ := run["conclusion"].(string)
		branch, _ := run["head_branch"].(string)
		createdAt, _ := run["created_at"].(string)
		runURL, _ := run["html_url"].(string)

		state := status
		if conclusion != "" {
			state = conclusion
		}
		bullet := "●"
		lineStyle := viewStyle
		if conclusion == "success" {
			lineStyle = successStyle
		} else if conclusion == "failure" {
			lineStyle = errorStyle
		}

		b.WriteString(lineStyle.Render(fmt.Sprintf("%s %s", bullet, name)))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  State: %s | Branch: %s\n", state, branch))
		b.WriteString(fmt.Sprintf("  Time: %s\n", createdAt))
		if runURL != "" {
			b.WriteString(fmt.Sprintf("  Run: %s\n", runURL))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("r: refresh security status"))
	return b.String()
}

func (m model) renderCreateAPIKeyForm() string {
	labels := []string{"Name", "Tenant ID"}
	var b strings.Builder
	b.WriteString(boxStyle.Render("Create API Key"))
	b.WriteString("\n\n")

	for i := 0; i < len(labels); i++ {
		indicator := "  "
		if i == m.apiKeyStep {
			indicator = "▶ "
		}

		b.WriteString(formLabelStyle.Render(fmt.Sprintf("%s%s:", indicator, labels[i])))
		b.WriteString("\n  ")
		b.WriteString(m.apiKeyInputs[i].View())
		b.WriteString("\n\n")
	}

	if m.apiKeyStep == 2 {
		b.WriteString(formLabelStyle.Render("▶ Submit") + "\n")
	} else {
		b.WriteString(viewStyle.Render("  Submit") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter/tab: next field | esc: cancel | enter on Submit: create"))

	return b.String()
}

func (m model) handleCreateAPIKeyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentView = viewAPIKeys
		m.apiKeyStep = 0
		for i := range m.apiKeyInputs {
			m.apiKeyInputs[i].Blur()
		}
		m.err = nil
		m.inputFocused = false
		return m, nil

	case "tab", "enter":
		if m.apiKeyStep == 2 {
			return m, m.submitCreateAPIKey()
		}

		m.apiKeyInputs[m.apiKeyStep].Blur()
		if m.apiKeyStep < 2 {
			m.apiKeyStep++
		}
		if m.apiKeyStep == 2 {
			m.inputFocused = false
			return m, nil
		}
		m.inputFocused = true
		return m, m.apiKeyInputs[m.apiKeyStep].Focus()
	}

	var cmd tea.Cmd
	m.apiKeyInputs[m.apiKeyStep], cmd = m.apiKeyInputs[m.apiKeyStep].Update(msg)
	return m, cmd
}

func (m model) submitCreateAPIKey() tea.Cmd {
	name := strings.TrimSpace(m.apiKeyInputs[0].Value())
	tenantID := strings.TrimSpace(m.apiKeyInputs[1].Value())

	if name == "" {
		return func() tea.Msg {
			return apiKeyCreateErrMsg{err: fmt.Errorf("name is required")}
		}
	}

	return func() tea.Msg {
		result, err := m.client.CreateAPIKey(name, tenantID)
		if err != nil {
			return apiKeyCreateErrMsg{err: err}
		}
		n := name
		if v, ok := result["name"].(string); ok {
			n = v
		}
		return apiKeyCreatedMsg{name: n}
	}
}

func (m model) renderDashboard() string {
	var b strings.Builder

	b.WriteString(boxStyle.Render("Gateway Status"))
	b.WriteString("\n")
	health, err := m.client.Health()
	if err != nil {
		b.WriteString(errorStyle.Render("  ✗ Gateway: DISCONNECTED"))
	} else {
		b.WriteString(successStyle.Render("  ✓ Gateway: " + health))
	}

	b.WriteString("\n\n")
	b.WriteString(boxStyle.Render("Statistics"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Agents:    %d\n", len(m.agents)))
	b.WriteString(fmt.Sprintf("  Resources: %d\n", len(m.resources)))
	b.WriteString(fmt.Sprintf("  HITL Pending: %d\n", len(m.hitlPending)))

	if len(m.agents) > 0 {
		b.WriteString("\n")
		b.WriteString(boxStyle.Render("Recent Agents"))
		b.WriteString("\n")
		max := 5
		if len(m.agents) < max {
			max = len(m.agents)
		}
		for i := 0; i < max; i++ {
			name, _ := m.agents[i]["name"].(string)
			status, _ := m.agents[i]["status"].(string)
			trust, _ := m.agents[i]["trust_score"].(float64)
			b.WriteString(fmt.Sprintf("  • %s [%s] trust: %.2f\n", name, status, trust))
		}
	}

	return b.String()
}

func (m model) renderAgents() string {
	var b strings.Builder

	if m.loading {
		return m.spinner.View() + " Loading agents..."
	}

	b.WriteString(boxStyle.Render(fmt.Sprintf("Agents (%d)", len(m.agents))))
	b.WriteString("\n\n")

	if len(m.agents) == 0 {
		return "No agents registered\n\n  Press n to register a new agent"
	}

	for i, agent := range m.agents {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "▶ "
		}

		name, _ := agent["name"].(string)
		owner, _ := agent["owner"].(string)
		status, _ := agent["status"].(string)
		trust, _ := agent["trust_score"].(float64)
		id, _ := agent["agent_id"].(string)

		line := fmt.Sprintf("%s%-20s %-15s %-10s trust: %.2f", cursor, name, owner, status, trust)
		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")

		if i == m.selectedIdx {
			b.WriteString(viewStyle.Render("  ID: " + id))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m model) renderResources() string {
	var b strings.Builder

	if m.loading {
		return m.spinner.View() + " Loading resources..."
	}

	b.WriteString(boxStyle.Render(fmt.Sprintf("Resources (%d)", len(m.resources))))
	b.WriteString("\n\n")

	if len(m.resources) == 0 {
		return "No resources registered"
	}

	for i, res := range m.resources {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "▶ "
		}

		name, _ := res["name"].(string)
		rtype, _ := res["resource_type"].(string)
		status, _ := res["status"].(string)
		risk, _ := res["risk_level"].(string)

		line := fmt.Sprintf("%s%-25s %-15s %-10s %s", cursor, name, rtype, status, risk)
		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderHITL() string {
	var b strings.Builder

	if m.loading {
		return m.spinner.View() + " Loading HITL approvals..."
	}

	b.WriteString(boxStyle.Render(fmt.Sprintf("HITL Pending Approvals (%d)", len(m.hitlPending))))
	b.WriteString("\n\n")

	if len(m.hitlPending) == 0 {
		return "No pending approvals"
	}

	for i, approval := range m.hitlPending {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "▶ "
		}

		agentID, _ := approval["agent_id"].(string)
		action, _ := approval["action"].(string)
		status, _ := approval["status"].(string)
		created, _ := approval["created_at"].(string)

		shortAgentID := truncateID(agentID, 8)
		line := fmt.Sprintf("%s%-12s %-20s %-10s %s", cursor, shortAgentID, action, status, created)
		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("a: approve | d: deny"))

	return b.String()
}

func (m model) renderAudit() string {
	var b strings.Builder

	if m.loading {
		return m.spinner.View() + " Loading audit logs..."
	}

	b.WriteString(boxStyle.Render(fmt.Sprintf("Audit Logs (%d)", len(m.auditLogs))))
	b.WriteString("\n\n")

	if len(m.auditLogs) == 0 {
		return "No audit logs"
	}

	for i, log := range m.auditLogs {
		cursor := "  "
		if i == m.selectedIdx {
			cursor = "▶ "
		}

		action, _ := log["action"].(string)
		status, _ := log["result_status"].(string)
		created, _ := log["created_at"].(string)
		trustBefore, _ := log["trust_score_before"].(float64)
		trustAfter, _ := log["trust_score_after"].(float64)

		line := fmt.Sprintf("%s%-20s %-10s trust: %.2f → %.2f  %s", cursor, action, status, trustBefore, trustAfter, created)
		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderRegisterForm() string {
	labels := []string{"Name", "Owner", "Capabilities", "Allowed Tools", "Max Budget (USD)"}

	var b strings.Builder
	b.WriteString(boxStyle.Render("Register New Agent"))
	b.WriteString("\n\n")

	for i := 0; i < len(labels); i++ {
		indicator := "  "
		if i == m.formIdx {
			indicator = "▶ "
		}

		b.WriteString(formLabelStyle.Render(fmt.Sprintf("%s%s:", indicator, labels[i])))
		b.WriteString("\n  ")
		b.WriteString(m.formInputs[i].View())
		b.WriteString("\n\n")
	}

	if m.formStep == stepSubmit {
		b.WriteString(formLabelStyle.Render("▶ Submit") + "\n")
	} else {
		b.WriteString(viewStyle.Render("  Submit") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter/tab: next field | esc: cancel | enter on Submit: register"))

	return b.String()
}
