package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/view"
)

// leftWidth is a quarter of the terminal, clamped so the panels stay
// readable on a narrow terminal and do not swallow a wide one.
func leftWidth(total int) int {
	w := total / 4
	switch {
	case w < 20:
		w = 20
	case w > 40:
		w = 40
	}
	if w > total-20 {
		w = total - 20
	}
	if w < 0 {
		w = 0
	}
	return w
}

// panel draws one bordered box with its title set into the top border, the
// way the spec's layout draws it. lipgloss v1 has no border title, so the top
// edge is drawn by hand and the box is rendered without one.
func panel(title string, focused bool, width, height int, body string) string {
	if width < 6 || height < 3 {
		return ""
	}
	edge, color := panelEdge(focused)
	inner := width - 2

	label := truncate(title, inner-2)
	fill := inner - 1 - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}
	top := lipgloss.NewStyle().Foreground(color).Render(
		edge.TopLeft+edge.Top) + titleStyle.Render(label) +
		lipgloss.NewStyle().Foreground(color).Render(strings.Repeat(edge.Top, fill)+edge.TopRight)

	lines := strings.Split(body, "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	lines = lines[:height-2]
	for i, l := range lines {
		lines[i] = lipgloss.NewStyle().Width(inner).MaxWidth(inner).Render(l)
	}
	box := lipgloss.NewStyle().
		Border(edge, false, true, true, true).
		BorderForeground(color).
		Width(inner).
		Render(strings.Join(lines, "\n"))

	return top + "\n" + box
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

// cursorFor marks the selected row, and marks it differently when its panel
// does not have focus, so there is never more than one live cursor.
func (m Model) cursorFor(focused, selected bool) string {
	switch {
	case selected && focused:
		return cursorStyle.Render("▸") + " "
	case selected:
		return dimStyle.Render("▸") + " "
	}
	return "  "
}

func (m Model) planRows(width, height int) string {
	rows := m.plans.rows()
	start, end := m.plans.window(height)
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.cursorFor(m.focus == focusPlans, m.plans.cursor == i))
		b.WriteString(truncate(rows[i].Key, width-4))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m Model) buildRows(width, height int) string {
	rows := m.builds.rows()
	start, end := m.builds.window(height)
	var b strings.Builder
	for i := start; i < end; i++ {
		bd := rows[i]
		b.WriteString(m.cursorFor(m.focus == focusBuilds, m.builds.cursor == i))
		b.WriteString(stateStyle(bd.State).Render(stateGlyph(bd.State)))
		fmt.Fprintf(&b, " #%d ", bd.Number)
		b.WriteString(dimStyle.Render(m.ago(bd.FinishedAt)))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// ago is relative time against the model's clock, so goldens are stable.
func (m Model) ago(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	return view.Ago(m.now().Sub(t))
}

func (m Model) presetRows(width, height int) string {
	rows := m.presets.rows()
	start, end := m.presets.window(height)
	var b strings.Builder
	for i := start; i < end; i++ {
		t := rows[i]
		b.WriteString(m.cursorFor(m.focus == focusPresets, m.presets.cursor == i))
		label := t.Name
		if w := width - 6 - lipgloss.Width(label); w > 6 && t.Plan != "" {
			label += dimStyle.Render("  " + truncate(t.Plan, w))
		}
		b.WriteString(truncate(label, width-4))
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m Model) leftColumn(width, height int) string {
	each := height / 3
	rest := height - 2*each
	return lipgloss.JoinVertical(lipgloss.Left,
		panel("1 Plans", m.focus == focusPlans, width, each, m.planRows(width, each-2)),
		panel("2 Builds", m.focus == focusBuilds, width, each, m.buildRows(width, each-2)),
		panel("3 Presets", m.focus == focusPresets, width, rest, m.presetRows(width, rest-2)),
	)
}

// mainPanel shows what the focused left panel points at. Part 2 replaces the
// placeholder body with the build detail and the stage and job tree.
func (m Model) mainPanel(width, height int) string {
	if b, ok := m.builds.selected(); ok {
		body := strings.Join([]string{
			fmt.Sprintf("%s   #%d   branch %s   %s",
				stateCell(b.State), b.Number, branchOrDefault(b), view.Duration(b.Duration)),
			dimStyle.Render(b.Reason),
		}, "\n")
		return panel(b.Key, m.focus == focusMain, width, height, body)
	}
	return panel("bam", m.focus == focusMain, width, height, dimStyle.Render("select a plan"))
}

func branchOrDefault(b provider.Build) string {
	if b.Branch == "" {
		return "default"
	}
	return b.Branch
}

// statusBar is one line: where we are on the left, what to press on the
// right. An error takes over the left half until it is dismissed.
func (m Model) statusBar(width int) string {
	left := fmt.Sprintf("%s · %s · %s", m.server, m.info.Version, m.user.Name)
	switch {
	case m.err != nil:
		left = errorStyle.Render(errorLine(m.err))
	case m.status != "":
		left = m.status
	}
	right := "?help  tab focus  l logs  o open  q quit"
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return truncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + statusStyle.Render(right)
}

// errorLine is the message to show in the status bar. Task 22 narrows it to
// an errs.Error's What.
func errorLine(err error) string { return err.Error() }
