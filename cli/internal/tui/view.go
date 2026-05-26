package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// View renders the screen UI
func (m model) View() string {
	if m.showBootSequence {
		return m.renderBootSequence()
	}

	// 1. Render sidebar (left column)
	sidebar := m.renderSidebar()

	// 2. Render active panel (right column top)
	var panel string
	switch m.currentView {
	case ViewOverview:
		panel = m.renderOverviewPanel()
	case ViewAgents:
		panel = m.renderAgentsPanel()
	case ViewPolicies:
		panel = m.renderPoliciesPanel()
	case ViewAudit:
		panel = m.renderAuditPanel()
	case ViewHITL:
		panel = m.renderHITLPanel()
	case ViewAirport:
		panel = m.renderAirportPanel()
	case ViewOnboarding:
		panel = m.renderOnboardingPanel()
	case ViewStatus:
		panel = m.renderStatusPanel()
	case ViewSkills:
		panel = m.renderSkillsPanel()
	case ViewFederation:
		panel = m.renderFederationPanel()
	case ViewAPIKeys:
		panel = m.renderAPIKeysPanel()
	case ViewSecurity:
		panel = m.renderSecurityPanel()
	}

	// 3. Command output (only if non-empty)
	var cmdOutputStr string
	if m.commandOutput != "" {
		cmdOutputStr = CommandOutputStyle.Render(m.commandOutput)
	}

	// 4. Command input bar
	var cmdPrompt string
	if m.currentView == ViewOnboarding {
		cmdPrompt = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorNeonCyan).
			Width(CmdW).
			Padding(0, 1).
			Render(m.onboardInput.View())
	} else {
		borderColor := ColorTextMuted
		if m.commandInput.Focused() {
			borderColor = ColorNeonGreen
		}
		cmdPrompt = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(borderColor).
			Width(CmdW).
			Padding(0, 1).
			Render(m.commandInput.View())
	}

	// 5. Footer (compact — sidebar already shows shortcuts)
	footer := FooterStyle.Render(
		"  [tab] cycle  ·  [↑↓ / jk] scroll  ·  [r] refresh  ·  [enter] run command  ·  [ctrl+c] quit",
	)

	// 6. Assemble right column: panel + cmd
	rightCol := lipgloss.JoinVertical(lipgloss.Left,
		panel,
		"",
		cmdOutputStr,
		cmdPrompt,
	)

	// 7. Assemble top row: sidebar | right
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebar,
		" ",
		rightCol,
	)

	// 8. Full layout
	fullLayout := lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		"",
		footer,
	)

	// Safe dimension check to prevent wrapping and layout desync
	if m.width < 88 || m.height < 24 {
		return AppStyle.Render(fullLayout)
	}
	return AppStyle.Render(
		lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, fullLayout),
	)
}

// ── Sidebar ──────────────────────────────────────────────────────────────────

func (m model) renderSidebar() string {
	// Status indicator
	statusDot := BlockedStyle.Render("●")
	statusLabel := MutedTextStyle.Render("DEMO")
	if m.connected {
		statusDot = VerifiedStyle.Render("●")
		statusLabel = VerifiedStyle.Render("LIVE")
	}

	// Title block
	titleLine := SidebarTitleStyle.Render("eyeVesa") + " " + statusDot + " " + statusLabel
	divider := SidebarDivStyle.Render(strings.Repeat("─", SidebarInnerW))

	// Count badges
	hitlPending := 0
	for _, r := range m.hitlRequests {
		if r.Status == "PENDING" {
			hitlPending++
		}
	}
	airportOnline := 0
	for _, a := range m.airportAgents {
		if strings.ToLower(a.Status) == "online" {
			airportOnline++
		}
	}
	apiActive := 0
	for _, k := range m.apiKeys {
		if k.Status == "ACTIVE" {
			apiActive++
		}
	}
	secFailed := 0
	for _, e := range m.securityEvents {
		if strings.ToLower(e.Conclusion) == "failure" {
			secFailed++
		}
	}

	// Nav items: (key, label, view, badge, badgeStyle)
	type navItem struct {
		key   string
		label string
		view  ViewIndex
		badge string
	}
	items := []navItem{
		{"1", "Overview", ViewOverview, ""},
		{"2", "Agents", ViewAgents, fmt.Sprintf("%d", len(m.agents))},
		{"3", "Policies", ViewPolicies, fmt.Sprintf("%d", len(m.policies))},
		{"4", "Audit", ViewAudit, fmt.Sprintf("%d", len(m.auditLogs))},
		{"5", "HITL", ViewHITL, func() string {
			if hitlPending > 0 {
				return fmt.Sprintf("⚡%d", hitlPending)
			}
			return ""
		}()},
		{"6", "Airport", ViewAirport, fmt.Sprintf("●%d", airportOnline)},
		{"7", "Onboard", ViewOnboarding, ""},
		{"8", "Status", ViewStatus, ""},
		{"9", "Skills", ViewSkills, fmt.Sprintf("%d", len(m.skills))},
		{"A", "Federation", ViewFederation, fmt.Sprintf("%d", len(m.trustBundles))},
		{"B", "API Keys", ViewAPIKeys, fmt.Sprintf("%d", apiActive)},
		{"C", "Security", ViewSecurity, func() string {
			if secFailed > 0 {
				return fmt.Sprintf("✗%d", secFailed)
			}
			return "✓"
		}()},
	}

	var lines []string
	lines = append(lines, titleLine)
	lines = append(lines, divider)

	for _, item := range items {
		isActive := m.currentView == item.view

		// Build label: "[key] Label"
		keyPart := fmt.Sprintf("[%s]", item.key)
		labelPart := item.label

		// Truncate label to fit
		maxLabelW := SidebarInnerW - 6 // [x] + space + badge area
		if len(labelPart) > maxLabelW {
			labelPart = labelPart[:maxLabelW]
		}

		// Badge (right-aligned)
		badge := ""
		if item.badge != "" {
			badge = item.badge
		}

		// Build the full line with padding
		textLen := 1 + len(keyPart) + 1 + len(labelPart) // ▶/space + [n] + space + label
		badgeLen := utf8.RuneCountInString(badge)
		pad := SidebarInnerW - textLen - badgeLen - 1
		if pad < 0 {
			pad = 0
		}
		spaces := strings.Repeat(" ", pad)

		var line string
		if isActive {
			prefix := SidebarActiveStyle.Render("▶" + keyPart + " " + labelPart)
			var badgeStr string
			if strings.HasPrefix(badge, "⚡") {
				badgeStr = BadgeHITLStyle.Render(badge)
			} else if strings.HasPrefix(badge, "✗") {
				badgeStr = BadgeHITLStyle.Render(badge)
			} else if badge != "" {
				badgeStr = BadgeOnlineStyle.Render(badge)
			}
			line = prefix + spaces + badgeStr
		} else {
			prefix := SidebarInactiveStyle.Render(" " + keyPart + " " + labelPart)
			var badgeStr string
			if strings.HasPrefix(badge, "⚡") {
				badgeStr = BadgeHITLStyle.Render(badge)
			} else if strings.HasPrefix(badge, "✗") {
				badgeStr = BadgeHITLStyle.Render(badge)
			} else if badge != "" {
				badgeStr = BadgeDefaultStyle.Render(badge)
			}
			line = prefix + spaces + badgeStr
		}
		lines = append(lines, line)
	}

	// Footer
	lines = append(lines, divider)
	gwText := "gw:localhost:8080"
	if len(gwText) > SidebarInnerW {
		gwText = gwText[:SidebarInnerW]
	}
	lines = append(lines, SidebarFooterStyle.Render(gwText))

	return SidebarStyle.Render(strings.Join(lines, "\n"))
}

// ── Boot sequence ─────────────────────────────────────────────────────────────

func (m model) renderBootSequence() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(TitleStyle.Render("  eyeVesa Native TUI v0.3.0") + "\n\n")

	for i := 0; i <= m.bootStep; i++ {
		if i < len(BootSequenceMessages) {
			if i == m.bootStep {
				sb.WriteString(fmt.Sprintf("  %s %s\n", m.spinner.View(), BootMsgStyle.Render(BootSequenceMessages[i])))
			} else {
				sb.WriteString(fmt.Sprintf("  ✓ %s\n", MutedTextStyle.Render(BootSequenceMessages[i])))
			}
		}
	}

	layout := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(ColorNeonGreen).
		Width(50).
		Padding(1, 3).
		Render(sb.String())

	return AppStyle.Render(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, layout))
}

// ── Panel helpers ─────────────────────────────────────────────────────────────

// panelBox wraps content in the standard panel border
func panelBox(content string) string {
	return PanelBoxStyle.Render(content)
}

// helpLine renders a muted command hint line
func helpLine(text string) string {
	return MutedTextStyle.Render("  " + text)
}

// ── Panel 1: Overview ────────────────────────────────────────────────────────

func (m model) renderOverviewPanel() string {
	// First agent snapshot
	agentName, agentDID, agentTrust, agentStatus := "hermes-ops", "did:eyevesa:agent:7F3A", "92%", "VERIFIED"
	agentStyle := VerifiedStyle
	if len(m.agents) > 0 {
		a := m.agents[0]
		agentName = a.Name
		agentDID = a.DID
		agentTrust = fmt.Sprintf("%d%%", a.Trust)
		agentStatus = a.Status
		switch agentStatus {
		case "BLOCKED":
			agentStyle = BlockedStyle
		case "RESTRICTED":
			agentStyle = RestrictedStyle
		default:
			agentStyle = VerifiedStyle
		}
	}

	hitlPending := 0
	for _, r := range m.hitlRequests {
		if r.Status == "PENDING" {
			hitlPending++
		}
	}
	airportOnline := len(m.airportAgents)

	policyMode := "DEMO"
	if m.connected {
		policyMode = "ENFORCING"
	}
	latestHash := "0x9fa...21c"
	if len(m.auditLogs) > 0 && m.auditLogs[0].Hash != "" {
		h := m.auditLogs[0].Hash
		if len(h) > 12 {
			h = h[:12]
		}
		latestHash = h
	}

	lines := []string{
		PanelTitleStyle.Render("Overview Dashboard"),
		"",
		PanelTitleStyle.Render("  Agent"),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Name:  "), ValueStyle.Render(agentName)),
		fmt.Sprintf("  %s %s", LabelStyle.Render("DID:   "), ValueStyle.Render(agentDID)),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Trust: "), ValueStyle.Render(agentTrust)),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Status:"), agentStyle.Render(agentStatus)),
		"",
		PanelTitleStyle.Render("  System"),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Policy:  "), EnforcingStyle.Render(policyMode)),
		fmt.Sprintf("  %s %s", LabelStyle.Render("HITL:    "), func() string {
			if hitlPending > 0 {
				return BlockedStyle.Render(fmt.Sprintf("%d PENDING", hitlPending))
			}
			return VerifiedStyle.Render("none pending")
		}()),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Airport: "), ValueStyle.Render(fmt.Sprintf("%d online", airportOnline))),
		fmt.Sprintf("  %s %s", LabelStyle.Render("Hash:    "), ValueStyle.Render(latestHash)),
		"",
		PanelTitleStyle.Render("  Recent Audit"),
	}

	limit := 4
	if len(m.auditLogs) < limit {
		limit = len(m.auditLogs)
	}
	for i := 0; i < limit; i++ {
		l := m.auditLogs[i]
		lines = append(lines, MutedTextStyle.Render(fmt.Sprintf("  [%s] %-10s %s", l.Timestamp, l.Agent, l.Event)))
	}

	return panelBox(strings.Join(lines, "\n"))
}

// ── Panel 2: Agents ──────────────────────────────────────────────────────────

func (m model) renderAgentsPanel() string {
	content := strings.Join([]string{
		PanelTitleStyle.Render(fmt.Sprintf("Agent Identity Registry  (%d)", len(m.agents))),
		"",
		m.agentsTable.View(),
		"",
		helpLine("check agent <name>  ·  list agents  ·  [↑↓] scroll"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 3: Policies ─────────────────────────────────────────────────────────

func (m model) renderPoliciesPanel() string {
	content := strings.Join([]string{
		PanelTitleStyle.Render(fmt.Sprintf("Policy & Guardrail Rules  (%d)", len(m.policies))),
		"",
		m.policiesTable.View(),
		"",
		helpLine("authorize <agent> <action>  ·  [↑↓] scroll"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 4: Audit ───────────────────────────────────────────────────────────

func (m model) renderAuditPanel() string {
	content := strings.Join([]string{
		PanelTitleStyle.Render(fmt.Sprintf("Non-Repudiation Audit Ledger  (%d)", len(m.auditLogs))),
		"",
		m.auditTable.View(),
		"",
		helpLine("audit  ·  audit <agent-id>  ·  [↑↓] scroll"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 5: HITL ─────────────────────────────────────────────────────────────

func (m model) renderHITLPanel() string {
	pending := 0
	for _, r := range m.hitlRequests {
		if r.Status == "PENDING" {
			pending++
		}
	}
	pendingStr := VerifiedStyle.Render("none pending")
	if pending > 0 {
		pendingStr = BadgeHITLStyle.Render(fmt.Sprintf("⚡ %d PENDING", pending))
	}

	content := strings.Join([]string{
		PanelTitleStyle.Render("Human-in-the-Loop") + "  " + pendingStr,
		"",
		m.hitlTable.View(),
		"",
		helpLine("hitl list  ·  hitl approve <id>  ·  hitl deny <id>  ·  [↑↓]"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 6: Airport ─────────────────────────────────────────────────────────

func (m model) renderAirportPanel() string {
	online := 0
	for _, a := range m.airportAgents {
		if strings.ToLower(a.Status) == "online" {
			online++
		}
	}
	onlineStr := VerifiedStyle.Render(fmt.Sprintf("● %d online", online))

	content := strings.Join([]string{
		PanelTitleStyle.Render("eyeVesa Airport") + "  " + onlineStr,
		"",
		m.airportTable.View(),
		"",
		helpLine("airport  ·  airport heartbeat <id>  ·  airport profile <id>  ·  [↑↓]"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 7: Onboarding ──────────────────────────────────────────────────────

func (m model) renderOnboardingPanel() string {
	steps := []string{"Agent Name", "Owner", "Capabilities", "Register"}

	var lines []string
	lines = append(lines, PanelTitleStyle.Render("Agent Onboarding Wizard"))
	lines = append(lines, "")

	// Step progress
	for i, s := range steps {
		var marker string
		switch {
		case i < m.onboardStep:
			marker = VerifiedStyle.Render("  ✓ " + s)
		case i == m.onboardStep:
			marker = EnforcingStyle.Render("  → " + s)
		default:
			marker = MutedTextStyle.Render("  ○ " + s)
		}
		lines = append(lines, marker)
	}
	lines = append(lines, "")

	// Entered values
	if m.onboardName != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", LabelStyle.Render("Name: "), ValueStyle.Render(m.onboardName)))
	}
	if m.onboardOwner != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", LabelStyle.Render("Owner:"), ValueStyle.Render(m.onboardOwner)))
	}
	if m.onboardCaps != "" {
		lines = append(lines, fmt.Sprintf("  %s %s", LabelStyle.Render("Caps: "), ValueStyle.Render(m.onboardCaps)))
	}
	lines = append(lines, "")

	// Result or hint
	if m.onboardStep < 3 {
		lines = append(lines, MutedTextStyle.Render("  Type value below and press [enter]"))
		lines = append(lines, MutedTextStyle.Render("  [esc] to cancel"))
	} else if m.onboardResult != "" {
		if strings.Contains(m.onboardResult, "ERROR") {
			lines = append(lines, BlockedStyle.Render("  "+m.onboardResult))
		} else {
			lines = append(lines, VerifiedStyle.Render("  "+m.onboardResult))
		}
		lines = append(lines, "")
		lines = append(lines, MutedTextStyle.Render("  [enter] to start over  ·  [esc] to cancel"))
	}

	return panelBox(strings.Join(lines, "\n"))
}

// ── Panel 8: Status ──────────────────────────────────────────────────────────

func (m model) renderStatusPanel() string {
	allUp := true
	for _, s := range m.svcStatus {
		if s.Status != "UP" {
			allUp = false
		}
	}
	overallStr := VerifiedStyle.Render("● ALL SYSTEMS OPERATIONAL")
	if !allUp {
		overallStr = BlockedStyle.Render("✗ DEGRADED")
	}

	content := strings.Join([]string{
		PanelTitleStyle.Render("System Status") + "  " + overallStr,
		"",
		m.statusTable.View(),
		"",
		helpLine("status  ·  [r] refresh"),
	}, "\n")
	return panelBox(content)
}

// ── Panel 9: Skills ──────────────────────────────────────────────────────────

func (m model) renderSkillsPanel() string {
	content := strings.Join([]string{
		PanelTitleStyle.Render(fmt.Sprintf("Skills Catalog  (%d)", len(m.skills))),
		"",
		m.skillsTable.View(),
		"",
		helpLine("skills list  ·  skills search <q>  ·  assign skill <a> <s>  ·  [↑↓]"),
	}, "\n")
	return panelBox(content)
}

// ── Panel A: Federation ──────────────────────────────────────────────────────

func (m model) renderFederationPanel() string {
	fedCount := 0
	for _, b := range m.trustBundles {
		if b.Federated == "YES" {
			fedCount++
		}
	}

	content := strings.Join([]string{
		PanelTitleStyle.Render("Federation & Trust Bundles") + "  " +
			VerifiedStyle.Render(fmt.Sprintf("◆ %d federated", fedCount)),
		"",
		m.federationTable.View(),
		"",
		helpLine("federation list  ·  federation verify <domain>  ·  [↑↓]"),
	}, "\n")
	return panelBox(content)
}

// ── Panel B: API Keys ────────────────────────────────────────────────────────

func (m model) renderAPIKeysPanel() string {
	active := 0
	for _, k := range m.apiKeys {
		if k.Status == "ACTIVE" {
			active++
		}
	}

	content := strings.Join([]string{
		PanelTitleStyle.Render("API Keys") + "  " +
			VerifiedStyle.Render(fmt.Sprintf("● %d active", active)),
		"",
		m.apiKeysTable.View(),
		"",
		helpLine("apikeys list  ·  apikeys create <n>  ·  apikeys revoke <id>  ·  [↑↓]"),
	}, "\n")
	return panelBox(content)
}

// ── Panel C: Security ─────────────────────────────────────────────────────────

func (m model) renderSecurityPanel() string {
	passed, failed := 0, 0
	for _, e := range m.securityEvents {
		if strings.ToLower(e.Conclusion) == "success" {
			passed++
		} else if strings.ToLower(e.Conclusion) == "failure" {
			failed++
		}
	}

	resultStr := VerifiedStyle.Render(fmt.Sprintf("✓ %d passed", passed))
	if failed > 0 {
		resultStr = BlockedStyle.Render(fmt.Sprintf("✗ %d failed / %d passed", failed, passed))
	}

	content := strings.Join([]string{
		PanelTitleStyle.Render("Security CI/CD Scans") + "  " + resultStr,
		"",
		m.securityTable.View(),
		"",
		helpLine("security  ·  [r] refresh  ·  requires GITHUB_TOKEN"),
	}, "\n")
	return panelBox(content)
}
