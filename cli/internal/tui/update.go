package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// BootStepMsg is sent to progress the boot sequence
type BootStepMsg int

func nextBootStepCmd(step int) tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(t time.Time) tea.Msg {
		return BootStepMsg(step)
	})
}

// Update loop for Bubble Tea
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// 1. Handle Boot Sequence
	case BootStepMsg:
		step := int(msg)
		if step < len(BootSequenceMessages) {
			m.bootStep = step
			m.bootMessage = BootSequenceMessages[step]
			return m, nextBootStepCmd(step + 1)
		}
		m.showBootSequence = false
		m.commandInput.Blur() // Blurred by default on startup
		// Fire live fetches on boot completion
		return m, tea.Batch(
			fetchAgentsCmd(m.apiClient),
			fetchPoliciesCmd(m.apiClient),
			fetchAuditCmd(m.apiClient),
			fetchHITLCmd(m.apiClient),
			fetchAirportCmd(m.apiClient),
			fetchSkillsCmd(m.apiClient),
			fetchStatusCmd(m.apiClient),
			fetchFederationCmd(m.apiClient),
			fetchAPIKeysCmd(m.apiClient),
			fetchSecurityCmd(m.apiClient),
		)

	// 2. Handle Live Fetch Results
	case fetchAgentsMsg:
		if msg.err == nil && len(msg.agents) > 0 {
			m.agents = msg.agents
			m.agentsTable = createAgentsTable(m.agents)
			m.connected = true
			m.gatewayStatus = "ACTIVE"
		} else if msg.err != nil {
			m.connected = false
			m.gatewayStatus = "DEMO MODE"
		}
		return m, nil

	case fetchPoliciesMsg:
		if msg.err == nil && len(msg.policies) > 0 {
			m.policies = msg.policies
			m.policiesTable = createPoliciesTable(m.policies)
		}
		return m, nil

	case fetchAuditMsg:
		if msg.err == nil && len(msg.auditLogs) > 0 {
			m.auditLogs = msg.auditLogs
			m.auditTable = createAuditTable(m.auditLogs)
		}
		return m, nil

	case fetchHITLMsg:
		if msg.err == nil {
			m.hitlRequests = msg.requests
			if len(m.hitlRequests) > 0 {
				m.hitlTable = createHITLTable(m.hitlRequests)
			}
		}
		return m, nil

	case fetchAirportMsg:
		if msg.err == nil && len(msg.agents) > 0 {
			m.airportAgents = msg.agents
			m.airportTable = createAirportTable(m.airportAgents)
		}
		return m, nil

	case fetchSkillsMsg:
		if msg.err == nil && len(msg.skills) > 0 {
			m.skills = msg.skills
			m.skillsTable = createSkillsTable(m.skills)
		}
		return m, nil

	case fetchStatusMsg:
		m.svcStatus = msg.services
		m.statusTable = createStatusTable(m.svcStatus)
		return m, nil

	case fetchFederationMsg:
		if msg.err == nil && len(msg.bundles) > 0 {
			m.trustBundles = msg.bundles
			m.federationTable = createFederationTable(m.trustBundles)
		}
		return m, nil

	case fetchAPIKeysMsg:
		if msg.err == nil && len(msg.keys) > 0 {
			m.apiKeys = msg.keys
			m.apiKeysTable = createAPIKeysTable(m.apiKeys)
		}
		return m, nil

	case fetchSecurityMsg:
		if msg.err == nil && len(msg.events) > 0 {
			m.securityEvents = msg.events
			m.securityTable = createSecurityTable(m.securityEvents)
		}
		return m, nil

	case registerAgentMsg:
		if msg.err != nil {
			m.onboardResult = fmt.Sprintf("ERROR: %v", msg.err)
		} else {
			m.onboardResult = msg.result
			// refresh agents list
			return m, fetchAgentsCmd(m.apiClient)
		}
		return m, nil

	// 3. Handle Spinner Tick
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// 4. Handle Keyboard Input
	case tea.KeyMsg:
		logToFile(fmt.Sprintf("Key pressed: %q view=%d", msg.String(), m.currentView))

		// ── Onboarding Wizard Mode ──────────────────────────────────────────
		if m.currentView == ViewOnboarding {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.onboardStep = 0
				m.onboardName = ""
				m.onboardOwner = ""
				m.onboardCaps = ""
				m.onboardResult = ""
				m.onboardInput.SetValue("")
				m.currentView = ViewOverview // Return to overview panel
				return m, nil
			case "enter", "ctrl+m":
				return m.handleOnboardEnter()
			}
			var cmd tea.Cmd
			m.onboardInput, cmd = m.onboardInput.Update(msg)
			return m, cmd
		}

		// ── Command Input Focused Mode ───────────────────────────────────────
		if m.commandInput.Focused() {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.commandInput.Blur()
				m.commandInput.SetValue("")
				m.commandOutput = ""
				return m, nil
			case "enter", "ctrl+m":
				val := m.commandInput.Value()
				if val != "" {
					output := ProcessCommand(m, val)
					if output == "__QUIT__" {
						return m, tea.Quit
					}
					m.commandOutput = output
					m.commandInput.SetValue("")
				}
				m.commandInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.commandInput, cmd = m.commandInput.Update(msg)
			return m, cmd
		}

		// ── Normal Mode Hotkeys (Command input is NOT focused) ────────────────
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "/":
			m.commandInput.Focus()
			m.commandInput.SetValue("")
			m.commandOutput = ""
			return m, nil

		// View shortcuts
		case "1":
			m.currentView = ViewOverview
			return m, nil
		case "2":
			m.currentView = ViewAgents
			return m, nil
		case "3":
			m.currentView = ViewPolicies
			return m, nil
		case "4":
			m.currentView = ViewAudit
			return m, nil
		case "5":
			m.currentView = ViewHITL
			return m, nil
		case "6":
			m.currentView = ViewAirport
			return m, nil
		case "7":
			m.currentView = ViewOnboarding
			return m, nil
		case "8":
			m.currentView = ViewStatus
			return m, nil
		case "9":
			m.currentView = ViewSkills
			return m, nil
		case "a", "A":
			m.currentView = ViewFederation
			return m, nil
		case "b", "B":
			m.currentView = ViewAPIKeys
			return m, nil
		case "c", "C":
			m.currentView = ViewSecurity
			return m, nil

		case "tab":
			m.currentView = (m.currentView + 1) % ViewCount
			return m, nil
		case "shift+tab":
			if m.currentView == 0 {
				m.currentView = ViewCount - 1
			} else {
				m.currentView--
			}
			return m, nil

		case "up", "k":
			switch m.currentView {
			case ViewAgents:
				m.agentsTable.MoveUp(1)
			case ViewPolicies:
				m.policiesTable.MoveUp(1)
			case ViewAudit:
				m.auditTable.MoveUp(1)
			case ViewHITL:
				m.hitlTable.MoveUp(1)
			case ViewAirport:
				m.airportTable.MoveUp(1)
			case ViewSkills:
				m.skillsTable.MoveUp(1)
			case ViewStatus:
				m.statusTable.MoveUp(1)
			case ViewFederation:
				m.federationTable.MoveUp(1)
			case ViewAPIKeys:
				m.apiKeysTable.MoveUp(1)
			case ViewSecurity:
				m.securityTable.MoveUp(1)
			}
			return m, nil

		case "down", "j":
			switch m.currentView {
			case ViewAgents:
				m.agentsTable.MoveDown(1)
			case ViewPolicies:
				m.policiesTable.MoveDown(1)
			case ViewAudit:
				m.auditTable.MoveDown(1)
			case ViewHITL:
				m.hitlTable.MoveDown(1)
			case ViewAirport:
				m.airportTable.MoveDown(1)
			case ViewSkills:
				m.skillsTable.MoveDown(1)
			case ViewStatus:
				m.statusTable.MoveDown(1)
			case ViewFederation:
				m.federationTable.MoveDown(1)
			case ViewAPIKeys:
				m.apiKeysTable.MoveDown(1)
			case ViewSecurity:
				m.securityTable.MoveDown(1)
			}
			return m, nil

		case "r", "ctrl+r":
			// Refresh live data
			m.commandOutput = "Refreshing live data..."
			return m, tea.Batch(
				fetchAgentsCmd(m.apiClient),
				fetchHITLCmd(m.apiClient),
				fetchAirportCmd(m.apiClient),
				fetchSkillsCmd(m.apiClient),
				fetchStatusCmd(m.apiClient),
			)
		}
		return m, nil

	// 5. Handle Terminal Sizing
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

// handleOnboardEnter advances the multi-step onboarding form
func (m model) handleOnboardEnter() (tea.Model, tea.Cmd) {
	val := m.onboardInput.Value()
	switch m.onboardStep {
	case 0:
		if val == "" {
			m.onboardResult = "Agent name is required."
			return m, nil
		}
		m.onboardName = val
		m.onboardStep = 1
		m.onboardInput.SetValue("")
		m.onboardInput.Placeholder = "owner (e.g. org:acme)..."
		m.onboardInput.Prompt = "  owner: "
	case 1:
		if val == "" {
			m.onboardResult = "Owner is required."
			return m, nil
		}
		m.onboardOwner = val
		m.onboardStep = 2
		m.onboardInput.SetValue("")
		m.onboardInput.Placeholder = "capabilities (comma-separated)..."
		m.onboardInput.Prompt = "  caps:  "
	case 2:
		m.onboardCaps = val
		m.onboardStep = 3
		m.onboardInput.SetValue("")
		m.onboardResult = fmt.Sprintf("Registering agent '%s'...", m.onboardName)
		return m, registerAgentCmd(m.apiClient, m.onboardName, m.onboardOwner, m.onboardCaps)
	case 3:
		// Reset after result shown
		m.onboardStep = 0
		m.onboardName = ""
		m.onboardOwner = ""
		m.onboardCaps = ""
		m.onboardResult = ""
		m.onboardInput.SetValue("")
		m.onboardInput.Placeholder = "agent name..."
		m.onboardInput.Prompt = "  name: "
	}
	return m, nil
}

// Blank line to ensure table import is used
var _ = table.DefaultStyles

func logToFile(msg string) {
	f, err := os.OpenFile("tui.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), msg))
	}
}
