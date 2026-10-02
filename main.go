package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
	"github.com/osman-butt/spring-init-tui/internal/tui"
)

func main() {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "spring-init-tui needs an interactive terminal")
		os.Exit(2)
	}

	final, err := tea.NewProgram(tui.New(initializr.NewClient())).Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if m, ok := final.(tui.Model); ok {
		switch {
		case m.Interrupted():
			os.Exit(130)
		case m.Err() != nil:
			os.Exit(1)
		}
	}
}
