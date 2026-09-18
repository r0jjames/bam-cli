package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/provider"
)

// inputMode says what the text input is collecting, if anything.
type inputMode int

const (
	inputNone inputMode = iota
	inputFilter
	inputSearch
)

// screen is what fills the terminal. The log screen takes the whole width.
type screen int

const (
	screenColumns screen = iota
	screenLogs
)

// focus is which panel takes keys. Main is the right-hand panel.
type focus int

const (
	focusPlans focus = iota
	focusBuilds
	focusPresets
	focusMain
)

// overlay is the modal on top, if any. At most one is open.
type overlay int

const (
	overlayNone overlay = iota
	overlayServers
	overlayProjects
	overlayBranches
	overlayHelp
	overlayError
)

var tabOrder = []focus{focusPlans, focusBuilds, focusPresets, focusMain}

// parent is where esc goes from here. Main came from Builds and Presets is a
// sibling of Builds, so neither steps through the tab order backwards.
func (f focus) parent() focus {
	switch f {
	case focusMain:
		return focusBuilds
	case focusBuilds, focusPresets:
		return focusPlans
	}
	return focusPlans
}

func (f focus) next() focus { return tabOrder[(int(f)+1)%len(tabOrder)] }
func (f focus) prev() focus { return tabOrder[(int(f)+len(tabOrder)-1)%len(tabOrder)] }

// Model is the whole UI. Update is a pure function of (Model, tea.Msg).
type Model struct {
	deps Deps

	width, height int

	screen  screen
	focus   focus
	overlay overlay

	server string
	info   provider.ServerInfo
	user   provider.User
	svc    *app.Service

	project    string // the project filter; empty means every project
	buildsPlan string // the plan (or branch plan) the Builds panel holds

	detail     *provider.Build
	expanded   map[string]bool
	treeCursor int

	plans   listState[provider.Plan]
	builds  listState[provider.Build]
	presets listState[app.TargetInfo]
	picker  listState[pickerItem]

	logs     logState
	input    textinput.Model
	inputFor inputMode

	// One generation per asynchronous stream. Bumping a stream's generation
	// abandons every request already in flight on it, so a slow response for
	// a server, project, branch, build or job the user has left cannot land
	// on the one they are looking at now.
	plansGen, buildsGen, logsGen int
	watchGen, followGen          int

	watchCancel context.CancelFunc
	watchCh     <-chan app.Event

	followCancel context.CancelFunc
	followLines  <-chan []string
	followDone   <-chan error

	now func() time.Time

	err    error
	status string
}

// New builds the initial model. It starts no work; Init does that.
func New(d Deps) Model {
	return Model{
		deps:    d,
		server:  d.Initial,
		screen:  screenColumns,
		focus:   focusPlans,
		plans:   newList(func(p provider.Plan) string { return p.Key + " " + p.Name }),
		builds:  newList(func(b provider.Build) string { return b.Key + " " + b.Branch + " " + b.Reason }),
		presets: newList(func(t app.TargetInfo) string { return t.Name + " " + t.Plan }),
		picker:  newList(func(p pickerItem) string { return p.Label + " " + p.Detail }),
		now:     time.Now,
		input:   textinput.New(),
	}
}

// loadPlans, loadBuilds and loadLogs own their stream's generation bump, so
// no call site can issue a request without invalidating the older ones.
func (m *Model) loadPlans() tea.Cmd {
	m.plansGen++
	m.plans.loading = true
	return loadPlansCmd(context.Background(), m.svc, m.project, m.plansGen)
}

// clear empties the panel first, for a switch to a different plan or branch:
// the rows on screen belong to what is being left, and Enter on one of them
// would open a build from the wrong plan. A plain refresh keeps them, so the
// cursor does not jump.
func (m *Model) loadBuilds(planKey string, clear bool) tea.Cmd {
	m.buildsGen++
	m.builds.loading = true
	if clear {
		m.builds.setItems(nil)
	}
	return loadBuildsCmd(context.Background(), m.svc, planKey, buildsPerPlan, m.buildsGen)
}

func (m *Model) loadLogs(b provider.Build, jobKey string, all bool) tea.Cmd {
	m.logsGen++
	return loadLogsCmd(context.Background(), m.svc, b, jobKey, all, m.logsGen)
}

// connected folds the start-up handshake's result into the model, so Init
// does not dial a second time.
func (m Model) connected(msg connectedMsg) Model {
	m.svc, m.info, m.user, m.server = msg.Svc, msg.Info, msg.User, msg.Alias
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	switch {
	case m.svc != nil:
		// Run's handshake already connected; go straight to the panels.
		m.plans.loading = true
		cmds = append(cmds, m.loadPlans())
	case m.deps.Connect != nil:
		cmds = append(cmds, connectCmd(m.deps, m.server))
	}
	if m.deps.Targets != nil {
		cmds = append(cmds, loadPresetsCmd(m.deps))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// The viewport holds its own width and height, so a resize has to
		// reach it or the log stays wrapped for the old terminal.
		m.logs.resize(m.width, m.logHeight())
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case connectedMsg:
		m.svc, m.info, m.user, m.server = msg.Svc, msg.Info, msg.User, msg.Alias
		m.err = nil
		return m, m.loadPlans()
	case plansLoadedMsg:
		if msg.Gen != m.plansGen {
			return m, nil
		}
		m.plans.loading = false
		m.plans.setItems(msg.Plans)
		return m, nil
	case buildsLoadedMsg:
		if msg.Gen != m.buildsGen {
			return m, nil
		}
		m.builds.loading = false
		m.buildsPlan = msg.PlanKey
		m.builds.setItems(msg.Builds)
		return m, nil
	case presetsLoadedMsg:
		m.presets.setItems(msg.Targets)
		return m, nil
	case projectsLoadedMsg:
		// The empty value is the "no filter" row, so one list both sets and
		// clears the filter.
		items := []pickerItem{{Label: "all projects", Value: ""}}
		for _, p := range msg.Projects {
			items = append(items, pickerItem{Label: p.Key, Value: p.Key, Detail: p.Name})
		}
		m.picker.setItems(items)
		m.picker.cursor = indexOf(items, m.project)
		return m, nil
	case branchesLoadedMsg:
		// The first row is the master plan itself, so one list both sets and
		// clears the branch.
		items := []pickerItem{{Label: "default branch", Value: msg.MasterKey}}
		for _, br := range msg.Branches {
			items = append(items, pickerItem{Label: br.ShortName, Value: br.Key, Detail: br.Name})
		}
		m.picker.setItems(items)
		m.picker.cursor = indexOf(items, m.buildsPlan)
		return m, nil
	case copiedMsg:
		m.status = "copied to clipboard (osc 52)"
		return m, nil
	case logChunkMsg:
		if msg.Gen != m.followGen {
			return m, nil
		}
		m.logs.appendLines(msg.Lines)
		return m, drainFollowCmd(m.followLines, m.followDone, m.followGen)
	case followEndedMsg:
		if msg.Gen != m.followGen {
			return m, nil
		}
		m.stopFollow()
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil
	case logsLoadedMsg:
		if msg.Gen != m.logsGen {
			return m, nil
		}
		m.logs.title, m.logs.jobKey, m.logs.url, m.logs.all = msg.Title, msg.JobKey, msg.URL, msg.All
		m.logs.setLines(m.width, m.logHeight(), msg.Lines)
		return m, nil
	case buildLoadedMsg:
		b := msg.Build
		m.detail = &b
		m.clampTree()
		return m, nil
	case watchEventMsg:
		if msg.Gen != m.watchGen {
			return m, nil
		}
		return m.handleWatchEvent(msg.Event)
	case watchClosedMsg:
		// A closed channel from an abandoned watch must not stop the current
		// one: that is a different watch, on a different build.
		if msg.Gen != m.watchGen {
			return m, nil
		}
		m.stopWatch()
		return m, nil
	case errMsg:
		m.err = msg.Err
		m.status = ""
		m.plans.loading, m.builds.loading = false, false
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.inputFor != inputNone {
		return m.handleInputKey(msg)
	}
	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
	}
	if m.screen == screenLogs {
		return m.handleLogKey(msg)
	}
	switch {
	case key.Matches(msg, keys.Quit):
		return m.quit()
	case key.Matches(msg, keys.Back):
		return m.back()
	case key.Matches(msg, keys.NextPanel):
		m.focus = m.focus.next()
	case key.Matches(msg, keys.PrevPanel):
		m.focus = m.focus.prev()
	case key.Matches(msg, keys.Panel1):
		m.focus = focusPlans
	case key.Matches(msg, keys.Panel2):
		m.focus = focusBuilds
	case key.Matches(msg, keys.Panel3):
		m.focus = focusPresets
	case key.Matches(msg, keys.Down):
		m.moveFocused(1)
	case key.Matches(msg, keys.Up):
		m.moveFocused(-1)
	case key.Matches(msg, keys.Top):
		m.jumpFocused(true)
	case key.Matches(msg, keys.Bottom):
		m.jumpFocused(false)
	case key.Matches(msg, keys.Enter):
		return m.drill()
	case key.Matches(msg, keys.Filter):
		return m.startInput()
	case key.Matches(msg, keys.NextMatch):
		m.logs.nextMatch(1)
	case key.Matches(msg, keys.PrevMatch):
		m.logs.nextMatch(-1)
	case key.Matches(msg, keys.Open):
		return m.openSelection()
	case key.Matches(msg, keys.Copy):
		return m.copySelection()
	case key.Matches(msg, keys.Logs):
		return m.openLogs("", false)
	case key.Matches(msg, keys.AllLogs):
		return m.openLogs("", true)
	case key.Matches(msg, keys.Help):
		m.overlay = overlayHelp
	case key.Matches(msg, keys.ExpandErr):
		if m.err == nil {
			return m, nil
		}
		m.overlay = overlayError
	case key.Matches(msg, keys.Refresh):
		return m.refresh()
	case key.Matches(msg, keys.Server):
		return m.openServerPicker()
	case key.Matches(msg, keys.Project):
		if m.svc == nil {
			return m, nil
		}
		m.overlay = overlayProjects
		m.picker.setQuery("")
		m.picker.setItems(nil)
		return m, loadProjectsCmd(context.Background(), m.svc)
	case key.Matches(msg, keys.Branch):
		p, ok := m.plans.selected()
		if !ok || m.svc == nil {
			return m, nil
		}
		m.overlay = overlayBranches
		m.picker.setQuery("")
		m.picker.setItems(nil)
		return m, loadBranchesCmd(context.Background(), m.svc, p.Key)
	}
	return m, nil
}

// handleInputKey owns every key while the user is typing, so q, l and j are
// characters rather than commands.
func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.inputFor = inputNone
		m.input.SetValue("")
		m.input.Blur()
		return m, nil
	case tea.KeyEnter:
		mode, q := m.inputFor, m.input.Value()
		m.inputFor = inputNone
		m.input.Blur()
		if mode == inputSearch {
			m.logs.search(q)
			return m, nil
		}
		m.applyFilter(q)
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// startInput opens the text input for the filter or the log search.
func (m Model) startInput() (tea.Model, tea.Cmd) {
	m.inputFor = inputFilter
	if m.screen == screenLogs {
		m.inputFor = inputSearch
	}
	m.input.SetValue("")
	m.input.Prompt = "/"
	m.input.Focus()
	return m, textinput.Blink
}

// applyFilter narrows whichever list has focus.
func (m *Model) applyFilter(q string) {
	switch m.focus {
	case focusPlans:
		m.plans.setQuery(q)
	case focusBuilds:
		m.builds.setQuery(q)
	case focusPresets:
		m.presets.setQuery(q)
	}
}

// handleOverlayKey keeps overlay keys from reaching the panels underneath.
func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		return m.back()
	case key.Matches(msg, keys.Quit):
		return m.quit()
	case key.Matches(msg, keys.Down):
		m.picker.move(1)
	case key.Matches(msg, keys.Up):
		m.picker.move(-1)
	case key.Matches(msg, keys.Top):
		m.picker.top()
	case key.Matches(msg, keys.Bottom):
		m.picker.bottom()
	case key.Matches(msg, keys.Enter):
		return m.chooseOverlay()
	}
	return m, nil
}

// refresh reloads whichever panel has focus. It is also the documented retry
// after a watch error, so it clears the error first. Presets is handled before
// the connection check, because rereading .bam.yaml needs no server.
func (m Model) refresh() (tea.Model, tea.Cmd) {
	m.err = nil
	if m.focus == focusPresets {
		if m.deps.Targets == nil {
			return m, nil
		}
		return m, loadPresetsCmd(m.deps)
	}
	if m.svc == nil {
		return m, nil
	}
	switch m.focus {
	case focusPlans:
		return m, m.loadPlans()
	case focusBuilds:
		if m.buildsPlan == "" {
			return m, nil
		}
		return m, m.loadBuilds(m.buildsPlan, false)
	case focusMain:
		if m.detail == nil {
			return m, nil
		}
		return m, reloadBuildCmd(context.Background(), m.svc, m.detail.Key)
	}
	return m, nil
}

func (m Model) openServerPicker() (tea.Model, tea.Cmd) {
	items := make([]pickerItem, 0, len(m.deps.Servers))
	for _, s := range m.deps.Servers {
		items = append(items, pickerItem{Label: s.Alias, Value: s.Alias, Detail: s.URL})
	}
	m.overlay = overlayServers
	m.picker.setQuery("")
	m.picker.setItems(items)
	m.picker.cursor = indexOf(items, m.server)
	return m, nil
}

func (m Model) chooseOverlay() (tea.Model, tea.Cmd) {
	it, ok := m.picker.selected()
	if !ok {
		m.overlay = overlayNone
		return m, nil
	}
	switch m.overlay {
	case overlayServers:
		m.overlay = overlayNone
		if it.Value == m.server {
			return m, nil
		}
		return m.switchServer(it.Value)
	case overlayProjects:
		m.overlay = overlayNone
		m.project = it.Value
		return m, m.loadPlans()
	case overlayBranches:
		m.overlay = overlayNone
		m.stopWatch()
		m.detail, m.expanded, m.treeCursor = nil, nil, 0
		m.buildsPlan = it.Value
		m.focus = focusBuilds
		return m, m.loadBuilds(it.Value, true)
	}
	m.overlay = overlayNone
	return m, nil
}

// switchServer drops everything that belonged to the old server: its build,
// its watch, its panels. Nothing keyed to one origin is shown under another.
func (m Model) switchServer(alias string) (tea.Model, tea.Cmd) {
	m.stopWatch()
	m.server, m.svc, m.detail = alias, nil, nil
	m.expanded, m.treeCursor = nil, 0
	m.buildsPlan = ""
	m.plans.setItems(nil)
	m.builds.setItems(nil)
	m.err = nil
	if m.deps.Connect == nil {
		return m, nil
	}
	return m, connectCmd(m.deps, alias)
}

// moveFocused moves exactly one cursor: the focused panel's.
func (m *Model) moveFocused(delta int) {
	switch m.focus {
	case focusPlans:
		m.plans.move(delta)
	case focusBuilds:
		m.builds.move(delta)
	case focusPresets:
		m.presets.move(delta)
	case focusMain:
		m.treeCursor += delta
		m.clampTree()
	}
}

func (m *Model) jumpFocused(top bool) {
	switch m.focus {
	case focusPlans:
		jump(&m.plans, top)
	case focusBuilds:
		jump(&m.builds, top)
	case focusPresets:
		jump(&m.presets, top)
	case focusMain:
		m.treeCursor = 0
		if !top {
			m.treeCursor = len(m.treeRows()) - 1
		}
		m.clampTree()
	}
}

func jump[T any](l *listState[T], top bool) {
	if top {
		l.top()
		return
	}
	l.bottom()
}

func (m *Model) clampTree() {
	n := len(m.treeRows())
	if m.treeCursor >= n {
		m.treeCursor = n - 1
	}
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
}

// treeRows is the stage and job tree of the open build, or nothing.
func (m Model) treeRows() []treeRow {
	if m.detail == nil {
		return nil
	}
	return buildTree(*m.detail, m.expanded)
}

func (m Model) selectedTreeRow() (treeRow, bool) {
	rows := m.treeRows()
	if m.treeCursor < 0 || m.treeCursor >= len(rows) {
		return treeRow{}, false
	}
	return rows[m.treeCursor], true
}

// drill is spec §4.1. Each row moves focus and starts the load its panel needs.
func (m Model) drill() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusPlans:
		p, ok := m.plans.selected()
		if !ok || m.svc == nil {
			return m, nil
		}
		m.focus = focusBuilds
		return m, m.loadBuilds(p.Key, true)
	case focusBuilds:
		b, ok := m.builds.selected()
		if !ok || m.svc == nil {
			return m, nil
		}
		m.focus = focusMain
		m.detail = &b
		m.expanded = defaultExpanded(b)
		m.treeCursor = 0
		return m.startWatch(b.Key)
	case focusPresets:
		t, ok := m.presets.selected()
		if !ok || m.svc == nil || t.Plan == "" {
			return m, nil
		}
		// Move the Plans cursor onto the preset's plan when it is listed, so
		// the panels agree about what the main panel is showing.
		for i, p := range m.plans.rows() {
			if p.Key == t.Plan {
				m.plans.cursor = i
				break
			}
		}
		m.focus = focusBuilds
		m.buildsGen++
		m.builds.loading = true
		m.builds.setItems(nil)
		// A preset may name a branch, and its builds live under the branch
		// plan, not the master. ResolvePlan is what turns the two into a plan
		// key, exactly as bam run does it.
		return m, resolveTargetBuildsCmd(context.Background(), m.svc, t.Name, m.buildsGen)
	case focusMain:
		row, ok := m.selectedTreeRow()
		if !ok {
			return m, nil
		}
		if row.Kind == rowStage {
			if m.expanded == nil {
				m.expanded = map[string]bool{}
			}
			m.expanded[row.Name] = !m.expanded[row.Name]
			m.clampTree()
			return m, nil
		}
		return m.openLogs(row.Key, false)
	}
	return m, nil
}

// handleWatchEvent folds one app.Event into the open build. The build in the
// event is the whole snapshot of that poll, so the tree follows it without a
// second request.
func (m Model) handleWatchEvent(e app.Event) (tea.Model, tea.Cmd) {
	if e.Build.Key != "" {
		b := e.Build
		m.detail = &b
		m.clampTree()
	}
	switch e.Type {
	case app.EventError:
		m.err = e.Err
		m.stopWatch()
		return m, nil
	case app.EventDone:
		m.stopWatch()
		if m.svc != nil && m.buildsPlan != "" {
			return m, m.loadBuilds(m.buildsPlan, false)
		}
		return m, nil
	}
	return m, watchCmd(m.watchCh, m.watchGen)
}

// startWatch begins watching key and cancels whatever was being watched
// before. At most one watch runs, so the UI polls at most one build and
// inherits app.Watch's own backoff. Ending a watch never stops the build.
func (m Model) startWatch(key string) (tea.Model, tea.Cmd) {
	m.stopWatch()
	if m.svc == nil {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.watchCancel = cancel
	m.watchCh = m.svc.Watch(ctx, key)
	return m, watchCmd(m.watchCh, m.watchGen)
}

// handleLogKey owns the keys while the log screen fills the terminal; the
// rest go to the viewport, which scrolls itself.
func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		return m.back()
	case key.Matches(msg, keys.Quit):
		return m.quit()
	case key.Matches(msg, keys.Bottom):
		m.logs.vp.GotoBottom()
		return m, nil
	case key.Matches(msg, keys.Top):
		m.logs.vp.GotoTop()
		return m, nil
	case key.Matches(msg, keys.Open):
		return m.openSelection()
	case key.Matches(msg, keys.Copy):
		return m.copySelection()
	case key.Matches(msg, keys.Help):
		m.overlay = overlayHelp
		return m, nil
	case key.Matches(msg, keys.ExpandErr):
		if m.err != nil {
			m.overlay = overlayError
		}
		return m, nil
	case key.Matches(msg, keys.Follow):
		return m.toggleFollow()
	case key.Matches(msg, keys.Filter):
		return m.startInput()
	case key.Matches(msg, keys.NextMatch):
		m.logs.nextMatch(1)
		return m, nil
	case key.Matches(msg, keys.PrevMatch):
		m.logs.nextMatch(-1)
		return m, nil
	}
	var cmd tea.Cmd
	m.logs.vp, cmd = m.logs.vp.Update(msg)
	return m, cmd
}

// openLogs takes the screen for the log viewport. An empty jobKey means the
// selected build's failed jobs, the same default bam logs uses.
func (m Model) openLogs(jobKey string, all bool) (tea.Model, tea.Cmd) {
	b, ok := m.currentBuild()
	if !ok || m.svc == nil {
		return m, nil
	}
	m.screen = screenLogs
	m.logs = logState{all: all}
	return m, m.loadLogs(b, jobKey, all)
}

// copiedMsg says the URL reached the terminal's clipboard.
type copiedMsg struct{}

// selectionURL is the Bamboo URL of whatever the focused panel points at, so
// o and y always act on what the user is looking at.
func (m Model) selectionURL() string {
	if m.screen == screenLogs && m.logs.url != "" {
		return m.logs.url
	}
	switch m.focus {
	case focusPlans:
		if p, ok := m.plans.selected(); ok {
			return p.URL
		}
	case focusBuilds:
		if b, ok := m.builds.selected(); ok {
			return b.URL
		}
	case focusMain:
		// A stage has no URL of its own, so the build's stands in.
		if row, ok := m.selectedTreeRow(); ok && row.URL != "" {
			return row.URL
		}
		if m.detail != nil {
			return m.detail.URL
		}
	}
	return ""
}

// openSelection and copySelection are shared by the columns and the log
// screen, so o and y mean the same thing on both.
func (m Model) openSelection() (tea.Model, tea.Cmd) {
	url := m.selectionURL()
	if url == "" || m.deps.Open == nil {
		return m, nil
	}
	open := m.deps.Open
	return m, func() tea.Msg {
		if err := open(url); err != nil {
			return errMsg{Err: err, Where: "open"}
		}
		return nil
	}
}

func (m Model) copySelection() (tea.Model, tea.Cmd) {
	url := m.selectionURL()
	if url == "" || m.deps.Clipboard == nil {
		return m, nil
	}
	w := m.deps.Clipboard
	return m, func() tea.Msg {
		if err := osc52(w, url); err != nil {
			return errMsg{Err: err, Where: "copy"}
		}
		return copiedMsg{}
	}
}

// toggleFollow starts or stops streaming the open job's log. Following polls
// that job only; it never touches the build.
func (m Model) toggleFollow() (tea.Model, tea.Cmd) {
	if m.logs.following {
		m.stopFollow()
		return m, nil
	}
	b, ok := m.currentBuild()
	if !ok || m.svc == nil || m.logs.jobKey == "" {
		return m, nil
	}
	job, ok := jobByKey(b, m.logs.jobKey)
	if !ok {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.followCancel = cancel
	m.logs.following = true
	m.followLines, m.followDone = followCmd(ctx, m.svc, b.Key, job, len(m.logs.lines))
	return m, drainFollowCmd(m.followLines, m.followDone, m.followGen)
}

func (m *Model) stopFollow() {
	if m.followCancel != nil {
		m.followCancel()
		m.followCancel = nil
	}
	m.followLines, m.followDone = nil, nil
	m.logs.following = false
	m.followGen++
}

func jobByKey(b provider.Build, key string) (provider.Job, bool) {
	for _, st := range b.Stages {
		for _, j := range st.Jobs {
			if j.Key == key {
				return j, true
			}
		}
	}
	return provider.Job{}, false
}

// currentBuild is the open build when there is one, otherwise whatever the
// Builds cursor points at.
// currentBuild is what l, a, o, y and f act on. The Builds cursor wins when
// Builds has focus, because esc leaves the detail open: without this, moving
// down one row and pressing l would show the previous build's log.
func (m Model) currentBuild() (provider.Build, bool) {
	if m.focus == focusBuilds {
		if b, ok := m.builds.selected(); ok {
			return b, true
		}
	}
	if m.detail != nil {
		return *m.detail, true
	}
	return m.builds.selected()
}

// back pops exactly one level, in the order spec §4.1 fixes.
func (m Model) back() (tea.Model, tea.Cmd) {
	switch {
	case m.overlay != overlayNone:
		// Closing the error overlay is how an error is dismissed.
		if m.overlay == overlayError {
			m.err = nil
		}
		m.overlay = overlayNone
		return m, nil
	case m.screen == screenLogs:
		m.stopFollow()
		m.screen = screenColumns
		return m, nil
	case m.focus != focusPlans:
		m.focus = m.focus.parent()
		return m, nil
	}
	return m.quit()
}

// quit cancels the watch on the way out. Cancelling a watch never stops the
// build; only bam build cancel does.
func (m Model) quit() (tea.Model, tea.Cmd) {
	m.stopWatch()
	m.stopFollow()
	return m, tea.Quit
}

func (m *Model) stopWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.watchCh = nil
	m.watchGen++
}

func (m Model) View() string {
	if m.width < 4 || m.height < 4 {
		return ""
	}
	if m.screen == screenLogs {
		return m.overlayView(m.logsView())
	}
	return m.columnsView()
}

// logHeight is the viewport's height: the terminal minus the header and the
// key line.
func (m Model) logHeight() int {
	h := m.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) columnsView() string {
	body := m.height - 1 // the status bar
	lw := leftWidth(m.width)
	return m.overlayView(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.leftColumn(lw, body),
			m.mainPanel(m.width-lw, body)),
		m.statusBar(m.width)))
}
