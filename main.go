// Command lazyandroid is a terminal UI for the `android` CLI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nicktriez/lazyandroid/internal/android"
	"github.com/Nicktriez/lazyandroid/internal/ui"
)

func main() {
	bin := os.Getenv("LAZYANDROID_BIN")
	p := tea.NewProgram(ui.New(android.Runner{Bin: bin}), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazyandroid:", err)
		os.Exit(1)
	}
}
