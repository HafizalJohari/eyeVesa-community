package cmd

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hafizaljohari/eyeVesa/cli/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long: `Launch an interactive terminal dashboard for eyeVesa.

Navigate using:
  - Tab/Shift+Tab: Switch between views
  - 1-4: Jump directly to Overview, Agents, Policies, or Audit
  - Enter: Run interactive commands
  - q/Ctrl+C: Quit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(tui.InitialModel(), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	addStartCommand(tuiCmd)
}

// parseList is a helper function used across the cmd package to parse comma-separated lists
func parseList(s string) []string {
	var list []string
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}
