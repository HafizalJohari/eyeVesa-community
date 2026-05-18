# eyeVesa TUI (Terminal User Interface)

> Interactive terminal dashboard for managing the AgentID Gateway.

---

## Overview

The eyeVesa TUI provides a visual, keyboard-driven interface for:

- **Dashboard**: Gateway status, statistics, recent agents
- **Agents**: Browse, inspect, and manage registered agents
- **Resources**: Browse and inspect registered resources
- **HITL**: Manage pending human-in-the-loop approvals
- **Audit**: View audit trail for agents

---

## Quick Start

```bash
# Start the TUI
./eyevesa tui

# Start with specific gateway
./eyevesa tui --gateway http://localhost:8080

# Start with config file
./eyevesa tui --config ~/.eyevesa/config.toml
```

---

## Keyboard Navigation

| Key | Action |
|-----|--------|
| `Tab` | Next view (Dashboard → Agents → Resources → HITL → Audit) |
| `Shift+Tab` | Previous view |
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `r` | Refresh current view |
| `a` | Approve HITL request (in HITL view) |
| `d` | Deny HITL request (in HITL view) |
| `q` | Quit |
| `Ctrl+C` | Quit |

---

## Views

### Dashboard

The dashboard provides an at-a-glance overview:

```
┌─ Gateway Status ────────────────────────────────┐
│ ✓ Gateway: ok                                   │
└─────────────────────────────────────────────────┘

┌─ Statistics ───────────────────────────────────┐
│ Agents:        47                               │
│ Resources:     12                               │
│ HITL Pending:  3                                │
└─────────────────────────────────────────────────┘

┌─ Recent Agents ─────────────────────────────────┐
│ • hermes-ops [active] trust: 0.95              │
│ • data-analyzer [active] trust: 1.00            │
│ • deployment-bot [active] trust: 0.87           │
└─────────────────────────────────────────────────┘
```

### Agents View

Browse all registered agents:

```
┌─ Agents (47) ────────────────────────────────────┐
│ ▶ hermes-ops        org:devops   active  0.95   │
│   ID: a1b2c3d4-...                              │
│   data-analyzer    org:analytics active  1.00  │
│   deployment-bot   org:platform   active  0.87 │
│   log-searcher     org:devops     active  0.92  │
└─────────────────────────────────────────────────┘
```

Operations:
- `↑/↓`: Navigate through agents
- `r`: Refresh list
- Selected agent shows full ID

### Resources View

Browse all registered resources:

```
┌─ Resources (12) ─────────────────────────────────┐
│ ▶ k8s-api          mcp_server    active   high  │
│   postgres-prod    mcp_server    active   medium │
│   zendesk-mcp      mcp_server    active   medium │
└─────────────────────────────────────────────────┘
```

### HITL View

Manage pending human-in-the-loop approvals:

```
┌─ HITL Pending Approvals (3) ────────────────────┐
│ ▶ a1b2c3d4...  k8s_deploy      pending  2m ago │
│   e5f6g7h8...  bank_transfer   pending  5m ago │
│   i9j0k1l2...  database_query  pending  1m ago │
└─────────────────────────────────────────────────┘

a: approve | d: deny
```

Operations:
- `a`: Approve selected request
- `d`: Deny selected request
- `r`: Refresh list

### Audit View

View audit trail for the selected agent:

```
┌─ Audit Logs (50) ─────────────────────────────────┐
│ ▶ k8s_deploy      allowed  trust: 0.92 → 0.93   │
│   log_search      allowed  trust: 0.91 → 0.92   │
│   bank_transfer   pending  trust: 0.93 → 0.93   │
│   database_query  denied   trust: 0.94 → 0.89   │
└──────────────────────────────────────────────────┘
```

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    eyeVesa TUI                      │
│                                                     │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐           │
│  │Dashboard│  │ Agents  │  │Resources │           │
│  └────┬────┘  └────┬────┘  └────┬────┘           │
│       │            │            │                  │
│  ┌────┴────┐  ┌────┴────┐  ┌────┴────┐           │
│  │  HITL   │  │  Audit  │  │ Delegate│           │
│  └────┬────┘  └────┬────┘  └────┬────┘           │
│       │            │            │                  │
└───────┼────────────┼────────────┼──────────────────┘
        │            │            │
        └────────────┼────────────┘
                     │
              ┌──────┴──────┐
              │  API Client │
              │  (HTTP/REST)│
              └──────┬──────┘
                     │
              ┌──────┴──────┐
              │   Gateway   │
              │ Control Plane│
              │   (:8080)    │
              └─────────────┘
```

---

## Technical Details

### Dependencies

```
github.com/charmbracelet/bubbletea  - TUI framework
github.com/charmbracelet/lipgloss   - Styling
github.com/charmbracelet/bubbles    - UI components
github.com/spf13/cobra              - CLI framework
```

### File Structure

```
cli/
├── cmd/
│   ├── tui.go              # TUI implementation
│   ├── root.go             # Root command
│   ├── init.go             # Agent registration
│   ├── agents.go           # Agent management
│   ├── resources.go        # Resource management
│   ├── authorize.go        # Authorization
│   ├── hitl.go             # HITL approvals
│   ├── delegate.go         # Delegation
│   ├── audit.go            # Audit trail
│   ├── mcp.go              # MCP operations
│   ├── discover.go         # Tool discovery
│   ├── doctor.go           # Diagnostics
│   └── config.go           # Configuration
├── internal/
│   ├── api/
│   │   └── client.go       # HTTP client for gateway
│   ├── config/
│   │   └── config.go      # Config file handling
│   └── crypto/
│       └── keys.go         # Ed25519 keypair ops
└── main.go
```

### Model-View-Update (MVU)

The TUI follows the Bubble Tea MVU pattern:

```go
// Model holds application state
type model struct {
    client       *api.Client
    currentView  view
    agents       []map[string]interface{}
    resources    []map[string]interface{}
    hitlPending  []map[string]interface{}
    selectedIdx  int
    // ...
}

// Init initializes the model
func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        m.loadAllData,
    )
}

// Update handles events
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        case "tab":
            m.currentView = (m.currentView + 1) % 5
        // ...
    }
    return m, nil
}

// View renders the UI
func (m model) View() string {
    // Build string output using lipgloss styling
}
```

---

## Customization

### Styling

The TUI uses `lipgloss` for styling. Modify the built-in styles:

```go
var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#7C3AED")).
        Padding(0, 1)

    selectedStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#10B981")).
        Bold(true)

    errorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#EF4444"))

    // Customize colors as needed
)
```

### Adding New Views

1. Add a new view constant:

```go
const (
    viewDashboard view = iota
    viewAgents
    viewResources
    viewHITL
    viewAudit
    viewYourNewView  // Add here
)
```

2. Add render function:

```go
func (m model) renderYourNewView() string {
    var b strings.Builder
    // Build your view
    return b.String()
}
```

3. Update the View() switch:

```go
switch m.currentView {
case viewDashboard:
    b.WriteString(m.renderDashboard())
// ...
case viewYourNewView:
    b.WriteString(m.renderYourNewView())
}
```

---

## Integration with CI/CD

For automated environments, use non-interactive commands instead:

```bash
# Non-interactive alternatives
./eyevesa agents list --output json
./eyevesa hitl list --output json
./eyevesa audit <agent-id> --output json
./eyevesa doctor

# For scripts
./eyevesa init --name my-agent --owner org:team --gateway http://gateway:8080
./eyevesa authorize --agent-id <id> --action deploy --output json
```

---

## Troubleshooting

### TUI Not Displaying Correctly

1. **Terminal size**: Ensure terminal is at least 80x24 characters
2. **Color support**: Set `TERM=xterm-256color` for full color support
3. **Unicode**: Some terminals may not render box-drawing characters

### Connection Issues

```bash
# Verify gateway is running
./eyevesa doctor

# Check specific gateway
./eyevesa tui --gateway http://your-gateway:8080
```

### Refresh Data

Press `r` in any view to reload data from the gateway.

---

## Comparison: TUI vs CLI Commands

| Task | TUI | CLI |
|------|-----|-----|
| Browse agents | `↑/↓` navigate | `./eyevesa agents list` |
| View single agent | Auto-select | `./eyevesa agents get <id>` |
| Approve HITL | `a` key | `./eyevesa hitl approve <id>` |
| View audit trail | Select agent → Tab to Audit | `./eyevesa audit <id>` |
| Quick overview | Dashboard view | `./eyevesa doctor` |
| Scripts/automation | Not applicable | Use CLI commands |

---

## Future Enhancements

- [ ] Real-time updates via WebSocket
- [ ] Trust score visualization (sparklines)
- [ ] Agent detail modal (press Enter)
- [ ] Resource action modal (invoke tools)
- [ ] Configuration editor
- [ ] Log streaming view
- [ ] Multiple gateway support
- [ ] Theme customization

---

## See Also

- [CLI Reference](cli-docs.md) - Full CLI command documentation
- [HOW_TO_USE.md](docs/HOW_TO_USE.md) - API usage guide
- [README.md](README.md) - Project overview