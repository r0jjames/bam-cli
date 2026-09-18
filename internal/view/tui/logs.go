package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// logState is the log screen. It keeps the raw lines as well as the viewport,
// so a search can find a match the viewport has scrolled past.
type logState struct {
	vp        viewport.Model
	title     string
	jobKey    string
	url       string
	all       bool
	lines     []string
	following bool
	ready     bool
}

func (l *logState) setLines(width, height int, lines []string) {
	if !l.ready {
		l.vp = viewport.New(width, height)
		l.ready = true
	}
	l.vp.Width, l.vp.Height = width, height
	l.lines = lines
	l.vp.SetContent(strings.Join(lines, "\n"))
	l.vp.GotoBottom()
}

// logsView is the whole terminal: a header, the viewport, a key line.
func (m Model) logsView() string {
	if m.height < 3 {
		return ""
	}
	source := "failed"
	if m.logs.all {
		source = "all"
	}
	shown := m.logs.vp.YOffset + m.logs.vp.Height
	if shown > len(m.logs.lines) {
		shown = len(m.logs.lines)
	}
	pos := fmt.Sprintf("%d/%d", shown, len(m.logs.lines))
	title := truncate(fmt.Sprintf("%s · logs · %s", m.logs.title, source), m.width-lipgloss.Width(pos)-2)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(pos)
	if gap < 1 {
		gap = 1
	}
	head := titleStyle.Render(title) + strings.Repeat(" ", gap) + dimStyle.Render(pos)

	keyLine := "esc back  / search  G end  f follow  n next failure  o open"
	if m.logs.following {
		keyLine = "following · " + keyLine
	}
	foot := truncate(statusStyle.Render(keyLine), m.width)
	if m.err != nil {
		foot = truncate(errorStyle.Render(errorLine(m.err)), m.width)
	}
	return strings.Join([]string{head, m.logs.vp.View(), foot}, "\n")
}
