package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
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
	rootCmd.AddCommand(tuiCmd)
}

type view int

const (
	viewDashboard view = iota
	viewAgents
	viewResources
	viewHITL
	viewAudit
)

type model struct {
	client       *api.Client
	currentView  view
	agents       []map[string]interface{}
	resources    []map[string]interface{}
	hitlPending  []map[string]interface{}
	auditLogs    []map[string]interface{}
	err          error
	loading      bool
	spinner      spinner.Model
	table        table.Model
	ready        bool
	selectedIdx  int
	statusMsg    string
	width        int
	height       int
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
)

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

	return model{
		client:      client,
		currentView: viewDashboard,
		spinner:     s,
		loading:     true,
		table:       t,
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.currentView = (m.currentView + 1) % 5
			m.selectedIdx = 0
			m.statusMsg = ""
			return m, nil
		case "shift+tab":
			if m.currentView == 0 {
				m.currentView = 4
			} else {
				m.currentView--
			}
			m.selectedIdx = 0
			m.statusMsg = ""
			return m, nil
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
			return m, nil
		case "down", "j":
			m.selectedIdx++
			return m, nil
		case "r":
			m.loading = true
			return m, m.refreshCurrentView
		case "a":
			if m.currentView == viewHITL && len(m.hitlPending) > 0 && m.selectedIdx < len(m.hitlPending) {
				if id, ok := m.hitlPending[m.selectedIdx]["approval_id"].(string); ok {
					return m, m.approveHITL(id)
				}
			}
		case "d":
			if m.currentView == viewHITL && len(m.hitlPending) > 0 && m.selectedIdx < len(m.hitlPending) {
				if id, ok := m.hitlPending[m.selectedIdx]["approval_id"].(string); ok {
					return m, m.denyHITL(id)
				}
			}
		}

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

	case refreshMsg:
		return m, tea.Batch(
			m.loadAgents,
			m.loadResources,
			m.loadHITL,
			m.loadAudit,
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
	}

	return m, nil
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

func (m model) approveHITL(approvalID string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.ApproveHILT(approvalID, "cli")
		if err != nil {
			m.err = err
			return nil
		}
		m.statusMsg = "Approved: " + approvalID[:8]
		return refreshMsg{}
	}
}

func (m model) denyHITL(approvalID string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.DenyHILT(approvalID, "cli")
		if err != nil {
			m.err = err
			return nil
		}
		m.statusMsg = "Denied: " + approvalID[:8]
		return refreshMsg{}
	}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Loading..."
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.String())
	b.WriteString("\n\n")

	// View tabs
	views := []string{"Dashboard", "Agents", "Resources", "HITL", "Audit"}
	for i, v := range views {
		if i == int(m.currentView) {
			b.WriteString(selectedStyle.Render("▶ " + v))
		} else {
			b.WriteString(viewStyle.Render("  " + v))
		}
		b.WriteString("  ")
	}
	b.WriteString("\n\n")

	// Handle errors
	if m.err != nil {
		b.WriteString(errorStyle.Render("❌ Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	// Main content based on current view
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
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ " + m.statusMsg))
	}

	// Help
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Tab: switch view | ↑↓: navigate | r: refresh | q: quit"))

	return b.String()
}

func (m model) renderDashboard() string {
	var b strings.Builder

	// Gateway status
	b.WriteString(boxStyle.Render("Gateway Status"))
	b.WriteString("\n")
	health, err := m.client.Health()
	if err != nil {
		b.WriteString(errorStyle.Render("  ✗ Gateway: DISCONNECTED"))
	} else {
		b.WriteString(successStyle.Render("  ✓ Gateway: " + health))
	}

	// Stats
	b.WriteString("\n\n")
	b.WriteString(boxStyle.Render("Statistics"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Agents:    %d\n", len(m.agents)))
	b.WriteString(fmt.Sprintf("  Resources: %d\n", len(m.resources)))
	b.WriteString(fmt.Sprintf("  HITL Pending: %d\n", len(m.hitlPending)))

	// Recent agents
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
		return "No agents registered"
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

		// Show full ID for selected
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

		line := fmt.Sprintf("%s%-12s %-20s %-10s %s", cursor, agentID[:8]+"...", action, status, created)
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