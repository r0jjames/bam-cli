package tui

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/view"
	"github.com/r0jjames/bam-cli/internal/view/style"
)

// homeState is what the Home screen keeps between frames (home spec §3-§4).
type homeState struct {
	view     homeView                  // which table Home shows
	sort     homeSort                  // the plans table's order
	live     map[string]provider.Build // builds the UI watches or started, by plan key
	loadedAt time.Time                 // the last load in which every project succeeded
	stale    bool                      // the last load, or part of it, failed
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

// homeFocus is the panel whose list Home shows. While Home is visible, focus
// points at it, so the shared key handling (movement, /, R, o, y) acts on
// the table's rows.
func (m Model) homeFocus() focus {
	if m.home.view == homePresets {
		return focusPresets
	}
	return focusPlans
}

// goHome shows Home again.
func (m Model) goHome() (tea.Model, tea.Cmd) {
	m.screen = screenHome
	m.focus = m.homeFocus()
	return m, nil
}

// handleHomeKey takes the keys that mean something else, or nothing, on
// Home. It reports handled=false for every other key, which then goes
// through handleKey's shared switch with focus on Home's list.
func (m Model) handleHomeKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.Enter):
		next, cmd := m.drillFromHome(focusBuilds)
		return next.(Model), cmd, true
	case key.Matches(msg, keys.Panel1):
		next, cmd := m.drillFromHome(focusPlans)
		return next.(Model), cmd, true
	case key.Matches(msg, keys.Panel2):
		next, cmd := m.drillFromHome(focusBuilds)
		return next.(Model), cmd, true
	case key.Matches(msg, keys.Panel3):
		next, cmd := m.drillFromHome(focusPresets)
		return next.(Model), cmd, true
	case key.Matches(msg, keys.Sort):
		m.setHomeSort(m.home.sort.next())
		return m, nil, true
	case key.Matches(msg, keys.Refresh):
		next, cmd := m.refreshHome()
		return next.(Model), cmd, true
	case key.Matches(msg, keys.NextPanel), key.Matches(msg, keys.PrevPanel),
		key.Matches(msg, keys.Logs), key.Matches(msg, keys.AllLogs), key.Matches(msg, keys.Follow),
		key.Matches(msg, keys.Cancel), key.Matches(msg, keys.Branch),
		key.Matches(msg, keys.NextMatch), key.Matches(msg, keys.PrevMatch):
		return m, nil, true // panel keys; Home has no panel for them
	}
	return m, nil, false
}

// drillFromHome opens the panels on the row's plan, then focuses want. It is
// enter on the Plans panel, so the builds load exactly as they do there.
func (m Model) drillFromHome(want focus) (tea.Model, tea.Cmd) {
	if _, ok := m.plans.selected(); !ok {
		return m, nil
	}
	m.screen = screenColumns
	m.focus = focusPlans
	next, cmd := m.drill()
	nm := next.(Model)
	nm.focus = want
	return nm, cmd
}

// setHomeSort re-sorts the plans and keeps the cursor on its plan.
func (m *Model) setHomeSort(s homeSort) {
	keep, had := m.plans.selected()
	m.home.sort = s
	m.plans.setOrder(planLess(s))
	if had {
		m.plans.selectFirst(func(p provider.Plan) bool { return p.Key == keep.Key })
	}
}

// refreshHome is r on Home: reload the plans now.
func (m Model) refreshHome() (tea.Model, tea.Cmd) {
	m.err = nil
	if m.svc == nil {
		return m, nil
	}
	return m, m.loadPlans()
}

// liveFor is the unfinished build the UI is watching or just started on
// planKey or one of its branches, or nil.
func (m Model) liveFor(planKey string) *provider.Build {
	for k, b := range m.home.live {
		if (k == planKey || isBranchOf(k, planKey)) && !b.State.Finished() {
			return &b
		}
	}
	return nil
}

// isBranchOf reports whether key is a branch plan of master: the master key
// followed by digits only (PROJ-PROV12 of PROJ-PROV).
func isBranchOf(key, master string) bool {
	rest, ok := strings.CutPrefix(key, master)
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// homeHeader is the line above the table: where, what and how it is sorted.
func (m Model) homeHeader() string {
	project := m.project
	if project == "" {
		project = "all"
		if m.svc != nil && len(m.svc.ProjectKeys()) == 0 {
			project = "all (none configured)"
		}
	}
	parts := []string{"bam", m.server, m.info.Version, m.user.Name, "project " + project,
		fmt.Sprintf("%d plans", m.plans.len()), "sort " + m.home.sort.label(homePlans)}
	if m.plans.query != "" {
		parts = append(parts, "filter /"+m.plans.query)
	}
	switch {
	case m.plans.loading && len(m.plans.items) > 0:
		parts = append(parts, "refreshing…")
	case m.home.stale && !m.home.loadedAt.IsZero():
		parts = append(parts, "stale "+ageShort(m.now().Sub(m.home.loadedAt)))
	case m.home.stale:
		parts = append(parts, "stale")
	}
	return " " + strings.Join(parts, " · ")
}

// planTable is the column header and the rows that fit in height lines.
func (m Model) planTable(width, height int) []string {
	rows := m.plans.rows()
	cols := planColumns(width, rows)
	start, end := m.plans.window(height - 1)
	cells := make([][]string, 0, end-start)
	for _, p := range rows[start:end] {
		cells = append(cells, planCells(cols, p, m.liveFor(p.Key), m.now()))
	}
	return tableLines(cols, cells, m.plans.cursor-start)
}

// homeView is the whole Home screen: header, table, status bar.
func (m Model) homeView() string {
	body := m.height - 2
	lines := m.planTable(m.width, body)
	if len(lines) > body {
		lines = lines[:body]
	}
	for len(lines) < body {
		lines = append(lines, "")
	}
	out := make([]string, 0, m.height)
	out = append(out, truncate(m.homeHeader(), m.width))
	for _, l := range lines {
		out = append(out, truncate(l, m.width))
	}
	out = append(out, m.statusBar(m.width))
	return m.overlayView(strings.Join(out, "\n"))
}
