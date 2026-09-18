package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// pickerItem is one row of any overlay list. Value is what choosing it means:
// a server alias, a project key or a branch plan key.
type pickerItem struct {
	Label  string
	Value  string
	Detail string
}

const overlayMaxWidth = 60

// overlayBox draws the modal frame, never wider than the terminal.
func overlayBox(title string, width int, body string) string {
	w := width - 8
	if w > overlayMaxWidth {
		w = overlayMaxWidth
	}
	if w < 12 {
		w = width - 2
	}
	if w < 4 {
		return ""
	}
	edge, color := panelEdge(true)
	return lipgloss.NewStyle().Border(edge).BorderForeground(color).Width(w - 2).
		Render(titleStyle.Render(truncate(title, w-2)) + "\n" + body)
}

func (m Model) pickerRows(height int) string {
	rows := m.picker.rows()
	start, end := m.picker.window(height)
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.cursorFor(true, m.picker.cursor == i))
		b.WriteString(rows[i].Label)
		if rows[i].Detail != "" {
			b.WriteString("  " + dimStyle.Render(rows[i].Detail))
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m Model) overlayTitle() string {
	switch m.overlay {
	case overlayServers:
		return "Servers"
	case overlayProjects:
		return "Projects"
	case overlayBranches:
		return "Branches"
	case overlayHelp:
		return "Keys"
	case overlayError:
		return "Error"
	}
	return ""
}

// overlayView centres the modal over the columns. It replaces the base rather
// than drawing on top of it, so no line can end up wider than the terminal.
func (m Model) overlayView(base string) string {
	if m.overlay == overlayNone {
		return base
	}
	box := overlayBox(m.overlayTitle(), m.width, m.pickerRows(m.height/2))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// indexOf is where value sits in items, or 0 when it is not there.
func indexOf(items []pickerItem, value string) int {
	for i, it := range items {
		if it.Value == value {
			return i
		}
	}
	return 0
}
