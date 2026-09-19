package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// confirmState is a pending yes/no question. Action is what yes does; it is a
// function so the overlay knows nothing about what it is confirming.
type confirmState struct {
	Prompt string
	Action func(Model) (tea.Model, tea.Cmd)
}

// confirmBody names what yes will do, so the answer is never a guess.
func (m Model) confirmBody(width int) string {
	return strings.Join([]string{
		truncate(m.confirm.Prompt, width),
		"",
		dimStyle.Render("y or enter to confirm, n or esc to cancel"),
	}, "\n")
}

// ask opens the confirmation. Nothing is sent until the answer.
func (m Model) ask(prompt string, action func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	m.overlay = overlayConfirm
	m.confirm = confirmState{Prompt: prompt, Action: action}
	return m, nil
}
