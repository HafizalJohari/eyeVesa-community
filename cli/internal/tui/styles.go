package tui

import "github.com/charmbracelet/lipgloss"

// ── Cyberpunk color palette ──────────────────────────────────────────────────
var (
	ColorBgMain    = lipgloss.Color("#000000") // Black
	ColorBgSidebar = lipgloss.Color("#0a0a0a") // Near-black sidebar
	ColorBgPanel   = lipgloss.Color("#050505") // Panel background

	ColorNeonGreen = lipgloss.Color("#39FF14") // Neon Green
	ColorNeonCyan  = lipgloss.Color("#00FFFF") // Neon Cyan
	ColorTextMain  = lipgloss.Color("#FFFFFF") // White
	ColorTextMuted = lipgloss.Color("#555555") // Gray
	ColorDanger    = lipgloss.Color("#FF3366") // Red
	ColorWarning   = lipgloss.Color("#FFEA00") // Yellow
)

// Layout constants
const (
	SidebarInnerW = 20 // inner content width of sidebar
	ContentW      = 54 // inner content width of panel  (56 - 2 padding)
	ContentPanelW = 56 // total Width() for panel border
	CmdW          = 56 // command bar full width
)

// ── Lipgloss styles ──────────────────────────────────────────────────────────
var (
	// App frame
	AppStyle = lipgloss.NewStyle().
			Background(ColorBgMain).
			Foreground(ColorTextMain)

	// Sidebar container
	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorNeonGreen).
			Background(ColorBgSidebar).
			Width(SidebarInnerW).
			Padding(0, 1)

	// Sidebar title line
	SidebarTitleStyle = lipgloss.NewStyle().
				Foreground(ColorNeonGreen).
				Bold(true)

	SidebarDivStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// Sidebar nav item — active
	SidebarActiveStyle = lipgloss.NewStyle().
				Foreground(ColorNeonGreen).
				Bold(true)

	// Sidebar nav item — inactive
	SidebarInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted)

	// Sidebar badge styles
	BadgeHITLStyle    = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	BadgeOnlineStyle  = lipgloss.NewStyle().Foreground(ColorNeonGreen).Bold(true)
	BadgeDefaultStyle = lipgloss.NewStyle().Foreground(ColorTextMuted)

	SidebarFooterStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				Italic(true)

	// Panel box
	PanelBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorNeonGreen).
			Width(ContentPanelW).
			Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Bold(true)

	// Key-Value
	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorTextMain)

	VerifiedStyle = lipgloss.NewStyle().
			Foreground(ColorNeonGreen).
			Bold(true)

	BlockedStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	RestrictedStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	EnforcingStyle = lipgloss.NewStyle().
			Foreground(ColorNeonGreen).
			Bold(true)

	MutedTextStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorNeonGreen).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Bold(true)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorNeonCyan).
				Bold(true).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorTextMuted)

	TableSelectedRowStyle = lipgloss.NewStyle().
				Foreground(ColorNeonGreen).
				Bold(true).
				Background(lipgloss.Color("#111111"))

	// Command input
	CommandPromptStyle = lipgloss.NewStyle().
				Foreground(ColorNeonGreen).
				Bold(true)

	CommandTextStyle = lipgloss.NewStyle().
				Foreground(ColorTextMain)

	CommandCursorStyle = lipgloss.NewStyle().
				Foreground(ColorNeonCyan)

	CommandOutputStyle = lipgloss.NewStyle().
				Foreground(ColorNeonCyan).
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(ColorTextMuted).
				Width(CmdW).
				Padding(0, 1)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	// Boot
	BootMsgStyle = lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Bold(true)

	BootSpinnerStyle = lipgloss.NewStyle().
				Foreground(ColorNeonGreen)

	// Legacy (kept for overview panel compat)
	ActivePanelBorder   = PanelBoxStyle
	InactivePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder(), true).
				BorderForeground(ColorTextMuted).
				Width(ContentPanelW).
				Padding(0, 1)

	HeaderBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(ColorNeonGreen).
			Width(CmdW).
			Align(lipgloss.Center).
			Padding(0, 1)
)
