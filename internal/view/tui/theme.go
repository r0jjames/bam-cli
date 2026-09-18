package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/view/style"
)

// stateColors repeats style's ANSI numbers as lipgloss colours. style stays
// the definition of what a state looks like; this is the only adapter, and
// theme_test.go fails if a state is added there without one here.
var stateColors = map[provider.State]lipgloss.Color{
	provider.StateSuccess:  lipgloss.Color("2"), // green
	provider.StateFailed:   lipgloss.Color("1"), // red
	provider.StateRunning:  lipgloss.Color("6"), // cyan
	provider.StateQueued:   lipgloss.Color("3"), // yellow
	provider.StateStopped:  lipgloss.Color("8"), // grey
	provider.StateNotBuilt: lipgloss.Color("8"), // dim
	provider.StateSkipped:  lipgloss.Color("8"), // dim
	provider.StateUnknown:  lipgloss.Color("5"), // magenta
}

func stateStyle(s provider.State) lipgloss.Style {
	c, ok := stateColors[s]
	if !ok {
		c = stateColors[provider.StateUnknown]
	}
	return lipgloss.NewStyle().Foreground(c)
}

// stateGlyph defers to style so there is one glyph table in the binary.
func stateGlyph(s provider.State) string { return style.Glyph(s) }

// stateCell is glyph plus label. A state is never signalled by colour alone.
func stateCell(s provider.State) string {
	return stateStyle(s).Render(style.Glyph(s) + " " + style.Label(s))
}

var (
	panelBorder        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	panelBorderFocused = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("4"))
	titleStyle         = lipgloss.NewStyle().Bold(true)
	dimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	statusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
)
