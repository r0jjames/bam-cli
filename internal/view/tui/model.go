package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/provider"
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

	plans   listState[provider.Plan]
	builds  listState[provider.Build]
	presets listState[app.TargetInfo]

	watchCancel context.CancelFunc

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
		now:     time.Now,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.deps.Connect != nil {
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
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case connectedMsg:
		m.svc, m.info, m.user, m.server = msg.Svc, msg.Info, msg.User, msg.Alias
		m.err = nil
		m.plans.loading = true
		return m, loadPlansCmd(context.Background(), m.svc, m.project)
	case plansLoadedMsg:
		m.plans.loading = false
		m.plans.setItems(msg.Plans)
		return m, nil
	case buildsLoadedMsg:
		m.builds.loading = false
		m.buildsPlan = msg.PlanKey
		m.builds.setItems(msg.Builds)
		return m, nil
	case presetsLoadedMsg:
		m.presets.setItems(msg.Targets)
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
	case key.Matches(msg, keys.Enter):
		return m.drill()
	}
	return m, nil
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
		m.builds.loading = true
		return m, loadBuildsCmd(context.Background(), m.svc, p.Key, buildsPerPlan)
	}
	return m, nil
}

// back pops exactly one level, in the order spec §4.1 fixes.
func (m Model) back() (tea.Model, tea.Cmd) {
	switch {
	case m.overlay != overlayNone:
		m.overlay = overlayNone
		return m, nil
	case m.screen == screenLogs:
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
	return m, tea.Quit
}

func (m *Model) stopWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
}

func (m Model) View() string {
	if m.width < 4 || m.height < 4 {
		return ""
	}
	if m.screen == screenLogs {
		return m.logsView()
	}
	return m.columnsView()
}

func (m Model) columnsView() string {
	body := m.height - 1 // the status bar
	lw := leftWidth(m.width)
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.leftColumn(lw, body),
			m.mainPanel(m.width-lw, body)),
		m.statusBar(m.width))
}

// logsView is filled in by Task 18.
func (m Model) logsView() string { return "" }
