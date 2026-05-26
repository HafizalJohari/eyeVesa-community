package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hafizaljohari/eyeVesa/cli/internal/tui"
)

func main() {
	// Parse CLI arguments
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		return
	}

	switch args[0] {
	case "tui":
		p := tea.NewProgram(tui.InitialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running eyeVesa TUI: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("eyeVesa CLI Tool (Standalone TUI)")
	fmt.Println("\nUsage:")
	fmt.Println("  go run ./cli/cmd/eyevesa [command]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  tui     Launch the interactive cyberpunk terminal UI")
	fmt.Println("  help    Show this help message")
}
