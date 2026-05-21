package main

import (
	"fmt"
	"os"

	"highway/tui"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if _, err := tea.NewProgram(tui.NewModel()).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
