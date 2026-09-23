package tui

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/view"
	"github.com/r0jjames/bam-cli/internal/view/style"
)

// homeState is what the Home screen keeps between frames (home spec §3-§4).
type homeState struct {
	loadedAt time.Time // the last load in which every project succeeded
	stale    bool      // the last load, or part of it, failed
}

// homeView is which table Home shows.
type homeView int

const (
	homePlans homeView = iota
	homePresets
)

// homeCol is a sortable column. The presets table reuses the values: key is
// the target name there, and name is the plan key.
type homeCol int

const (
	colKey homeCol = iota
	colName
	colState
	colAge
)

// homeColNames are the column names :sort accepts, per table, in homeCol order.
var homeColNames = map[homeView][]string{
	homePlans:   {"key", "name", "state", "age"},
	homePresets: {"target", "plan", "state", "age"},
}

// homeSort is a column and a direction. desc reverses the column's natural
// direction (home spec §3.4).
type homeSort struct {
	col  homeCol
	desc bool
}

// next is what s does: the following column, in its natural direction.
func (s homeSort) next() homeSort { return homeSort{col: (s.col + 1) % 4} }

func (s homeSort) label(v homeView) string {
	arrow := "↑"
	if s.desc {
		arrow = "↓"
	}
	return homeColNames[v][s.col] + arrow
}

// parseSort reads a :sort argument: a column of view v, optionally with a
// leading "-" to reverse it.
func parseSort(v homeView, arg string) (homeSort, error) {
	name := strings.TrimPrefix(arg, "-")
	for i, n := range homeColNames[v] {
		if n == name {
			return homeSort{col: homeCol(i), desc: strings.HasPrefix(arg, "-")}, nil
		}
	}
	return homeSort{}, fmt.Errorf("unknown sort column %q; columns: %s", name, strings.Join(homeColNames[v], " "))
}

// stateRank orders the state column: what needs attention first, a plan
// that never built last.
func stateRank(b *provider.BuildSummary) int {
	if b == nil {
		return 8
	}
	switch b.State {
	case provider.StateFailed:
		return 0
	case provider.StateRunning:
		return 1
	case provider.StateQueued:
		return 2
	case provider.StateStopped:
		return 3
	case provider.StateNotBuilt:
		return 4
	case provider.StateSkipped:
		return 5
	case provider.StateSuccess:
		return 7
	}
	return 6
}

func finishedAt(b *provider.BuildSummary) time.Time {
	if b == nil {
		return time.Time{}
	}
	return b.FinishedAt
}

// compareAge puts the newest first and a missing time last.
func compareAge(a, b time.Time) int {
	switch {
	case a.Equal(b):
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	case a.After(b):
		return -1
	}
	return 1
}

func planKeyCompare(a, b provider.Plan) int {
	if c := strings.Compare(a.ProjectKey, b.ProjectKey); c != 0 {
		return c
	}
	return strings.Compare(a.Key, b.Key)
}

// planLess is the plans table's order under s. Ties break on project then
// key, ascending, so the order is total and a refresh cannot shuffle rows.
func planLess(s homeSort) func(a, b provider.Plan) bool {
	return func(a, b provider.Plan) bool {
		var c int
		switch s.col {
		case colName:
			c = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case colState:
			c = cmp.Compare(stateRank(a.LastBuild), stateRank(b.LastBuild))
		case colAge:
			c = compareAge(finishedAt(a.LastBuild), finishedAt(b.LastBuild))
		default:
			c = planKeyCompare(a, b)
		}
		if c != 0 {
			if s.desc {
				return c > 0
			}
			return c < 0
		}
		return planKeyCompare(a, b) < 0
	}
}

// planMatchText is what / matches a plan on: key, name, project and the last
// build's state word, so /fail keeps the failed plans.
func planMatchText(p provider.Plan) string {
	txt := p.ProjectKey + " " + p.Key + " " + p.Name
	if p.LastBuild != nil {
		txt += " " + style.Label(p.LastBuild.State)
	}
	return txt
}

// shortBy is the BY column: who or what started a build, from its reason.
func shortBy(reason string) string {
	r := strings.TrimSpace(reason)
	switch {
	case r == "":
		return "–"
	case strings.HasPrefix(r, "Manual run by "):
		return strings.TrimPrefix(r, "Manual run by ")
	case strings.HasPrefix(r, "Scheduled"):
		return "sched"
	case strings.HasPrefix(r, "Changes by"), strings.HasPrefix(r, "Code has changed"):
		return "commit"
	case strings.HasPrefix(r, "Child of"), strings.HasPrefix(r, "Dependant of"):
		return "dep"
	}
	return strings.Fields(r)[0]
}

// ageShort is the AGE column: now, 12m, 3h, 2d.
func ageShort(d time.Duration) string {
	if d < time.Minute {
		return "now"
	}
	return strings.TrimSuffix(view.Ago(d), " ago")
}

// tableCol is one column of a Home table.
type tableCol struct {
	title string
	width int
	right bool
}

// planColumns fits the plans table to width (home spec §3.2). NAME takes
// what is left; BY goes below 100 columns and PROJECT below 80.
func planColumns(width int, rows []provider.Plan) []tableCol {
	keyW, projW := len("KEY"), len("PROJECT")
	for _, p := range rows {
		keyW = max(keyW, lipgloss.Width(p.Key))
		projW = max(projW, lipgloss.Width(p.ProjectKey))
	}
	keyW, projW = min(keyW, 24), min(projW, 10)

	var cols []tableCol
	if width >= 80 {
		cols = append(cols, tableCol{title: "PROJECT", width: projW})
	}
	cols = append(cols,
		tableCol{title: "KEY", width: keyW},
		tableCol{title: "NAME"},
		tableCol{title: "STATE", width: 11},
		tableCol{title: "#", width: 6, right: true},
		tableCol{title: "AGE", width: 5})
	if width >= 100 {
		cols = append(cols, tableCol{title: "BY", width: 12})
	}
	return fillWidth(cols, width, "NAME")
}

// fillWidth gives the flexible column what the others leave of width: two
// columns of cursor, two between columns.
func fillWidth(cols []tableCol, width int, flexible string) []tableCol {
	used := 2 + 2*(len(cols)-1)
	for _, c := range cols {
		used += c.width
	}
	for i := range cols {
		if cols[i].title == flexible {
			cols[i].width = max(width-used, 4)
		}
	}
	return cols
}

// buildCells is STATE, #, AGE and BY for a plan's last build. A live build
// the UI is watching or just started wins while it has not finished (home
// spec §4.4).
func buildCells(last *provider.BuildSummary, live *provider.Build, now time.Time) (state, num, age, by string) {
	switch {
	case live != nil && !live.State.Finished():
		return stateCell(live.State), strconv.Itoa(live.Number), "now", shortBy(live.Reason)
	case last != nil:
		age = "–"
		if !last.FinishedAt.IsZero() {
			age = ageShort(now.Sub(last.FinishedAt))
		}
		return stateCell(last.State), strconv.Itoa(last.Number), age, shortBy(last.Reason)
	}
	return "–", "–", "–", "–"
}

// planCells is one plans-table row, in cols order.
func planCells(cols []tableCol, p provider.Plan, live *provider.Build, now time.Time) []string {
	state, num, age, by := buildCells(p.LastBuild, live, now)
	out := make([]string, len(cols))
	for i, c := range cols {
		switch c.title {
		case "PROJECT":
			out[i] = p.ProjectKey
		case "KEY":
			out[i] = p.Key
		case "NAME":
			out[i] = p.Name
		case "STATE":
			out[i] = state
		case "#":
			out[i] = num
		case "AGE":
			out[i] = age
		case "BY":
			out[i] = by
		}
	}
	return out
}

// tableLines is a header line and one line per row. cursor is the row index
// that gets the ▸ marker; -1 marks none.
func tableLines(cols []tableCol, cells [][]string, cursor int) []string {
	head := make([]string, len(cols))
	for i, c := range cols {
		head[i] = c.title
	}
	lines := []string{"  " + titleStyle.Render(joinCells(cols, head))}
	for r, row := range cells {
		mark := "  "
		if r == cursor {
			mark = cursorStyle.Render("▸") + " "
		}
		lines = append(lines, mark+joinCells(cols, row))
	}
	return lines
}

func joinCells(cols []tableCol, cells []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		s := cut(cells[i], c.width)
		pad := strings.Repeat(" ", max(c.width-lipgloss.Width(s), 0))
		if c.right {
			parts[i] = pad + s
		} else {
			parts[i] = s + pad
		}
	}
	return strings.TrimRight(strings.Join(parts, "  "), " ")
}

// cut shortens s to w columns, ending in … when it had to cut.
func cut(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return truncate(s, w-1) + "…"
}
