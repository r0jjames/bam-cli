package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

	plans   listState[provider.Plan]
	builds  listState[provider.Build]
	presets listState[app.TargetInfo]

	watchCancel context.CancelFunc

	err    error
	status string
}

// New builds the initial model. It starts no work; Init does that.
func New(d Deps) Model {
	return Model{deps: d, server: d.Initial, screen: screenColumns, focus: focusPlans}
}

func (m Model) Init() tea.Cmd { return connectCmd(m.deps, m.server) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
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

// View is filled in by Task 7.
func (m Model) View() string { return "" }

// listState and connectCmd arrive in Tasks 5 and 6.
type listState[T any] struct{ items []T }

func connectCmd(Deps, string) tea.Cmd { return nil }
