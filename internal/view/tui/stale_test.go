package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

// The tests here pin the Copilot review findings on PR #5: a response, or a
// channel close, that belongs to something the user has already left must not
// land on what they are looking at now.

// TestStaleWatchCloseDoesNotStopTheCurrentWatch is the sharpest of them: the
// abandoned watch's command is still scheduled, and its watchClosedMsg used
// to cancel the watch that replaced it.
func TestStaleWatchCloseDoesNotStopTheCurrentWatch(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m, _ = send(m, buildsLoadedMsg{Gen: m.buildsGen, PlanKey: "PROJ-BUILD", Builds: []provider.Build{
		{Key: "PROJ-BUILD-44", Number: 44}, {Key: "PROJ-BUILD-43", Number: 43},
	}})
	m.focus = focusBuilds

	m, _ = send(m, mkKey("enter")) // watch A
	staleGen := m.watchGen

	m.focus = focusBuilds
	m, _ = send(m, mkKey("j"))
	m, _ = send(m, mkKey("enter")) // watch B
	require.NotEqual(t, staleGen, m.watchGen)
	require.NotNil(t, m.watchCancel)

	m, cmd := send(m, watchClosedMsg{Gen: staleGen})
	require.NotNil(t, m.watchCancel, "watch B must survive watch A's close")
	require.Nil(t, cmd)
	m.stopWatch()
}

// TestStaleWatchEventDoesNotOverwriteTheCurrentBuild.
func TestStaleWatchEventDoesNotOverwriteTheCurrentBuild(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.detail = ptr(sampleBuild())
	m.watchGen = 7

	old := sampleBuild()
	old.Key, old.State = "PROJ-BUILD-43", provider.StateSuccess
	m, _ = send(m, watchEventMsg{Gen: 6, Event: app.Event{Type: app.EventState, Build: old}})
	require.Equal(t, "PROJ-BUILD-44", m.detail.Key)
}

// TestStaleDoneDoesNotRefreshOrStop.
func TestStaleDoneDoesNotRefreshOrStop(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.buildsPlan = "PROJ-BUILD"
	m.watchGen = 3
	stopped := false
	m.watchCancel = func() { stopped = true }

	m, cmd := send(m, watchEventMsg{Gen: 2, Event: app.Event{Type: app.EventDone, State: provider.StateFailed}})
	require.False(t, stopped)
	require.Nil(t, cmd)
	require.NotNil(t, m.watchCancel)
}

// TestStalePlansResponseIsDropped: switching project must not be undone by the
// previous project's slower reply.
func TestStalePlansResponseIsDropped(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.plans.setItems([]provider.Plan{{Key: "OPS-NIGHTLY"}})
	stale := m.plansGen

	m.plansGen++ // as a project switch would
	m, _ = send(m, plansLoadedMsg{Gen: stale, Plans: []provider.Plan{{Key: "PROJ-BUILD"}, {Key: "PROJ-PROV"}}})
	require.Equal(t, 1, m.plans.len())
	sel, _ := m.plans.selected()
	require.Equal(t, "OPS-NIGHTLY", sel.Key)
}

// TestStaleBuildsResponseIsDropped, the same during a branch switch.
func TestStaleBuildsResponseIsDropped(t *testing.T) {
	m := testModel()
	m.svc = testService()
	stale := m.buildsGen
	m.buildsGen++
	m, _ = send(m, buildsLoadedMsg{Gen: stale, PlanKey: "PROJ-BUILD", Builds: []provider.Build{{Key: "PROJ-BUILD-44"}}})
	require.Equal(t, 0, m.builds.len())
	require.Equal(t, "", m.buildsPlan)
}

// TestStaleLogsResponseIsDropped: opening a second log before the first
// arrives must not show the first.
func TestStaleLogsResponseIsDropped(t *testing.T) {
	m := logModel()
	stale := m.logsGen
	m.logsGen++
	m, _ = send(m, logsLoadedMsg{Gen: stale, JobKey: "OTHER", Title: "OTHER", Lines: []string{"wrong log"}})
	require.Equal(t, "PROJ-BUILD-INT-44", m.logs.title)
	require.NotContains(t, m.logs.lines, "wrong log")
}

// TestStaleFollowChunkIsDropped.
func TestStaleFollowChunkIsDropped(t *testing.T) {
	m := logModel()
	before := len(m.logs.lines)
	m.followGen = 4
	m, _ = send(m, logChunkMsg{Gen: 3, Lines: []string{"from a job we left"}})
	require.Len(t, m.logs.lines, before)
}

// TestBranchSwitchClearsTheBuildsRows: the rows on screen belong to the branch
// being left, and Enter on one of them would open the wrong build.
func TestBranchSwitchClearsTheBuildsRows(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{{Key: "PROJ-PROV"}}})
	require.Positive(t, m.builds.len())

	m, _ = send(m, mkKey("b"))
	m, _ = send(m, branchesLoadedMsg{MasterKey: "PROJ-PROV", Branches: []provider.Branch{
		{Key: "PROJ-PROV12", ShortName: "develop", PlanKey: "PROJ-PROV"},
	}})
	m, _ = send(m, mkKey("j"))
	m, _ = send(m, mkKey("enter"))
	require.Equal(t, 0, m.builds.len(), "the old branch's builds are gone while the new ones load")
}

// TestRefreshKeepsTheBuildsRowsAndTheCursor: a refresh is not a switch.
func TestRefreshKeepsTheBuildsRowsAndTheCursor(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.buildsPlan = "PROJ-BUILD"
	m.focus = focusBuilds
	m.builds.move(1)

	m, _ = send(m, mkKey("r"))
	require.Equal(t, 2, m.builds.len())
	require.Equal(t, 1, m.builds.cursor)
}

// TestCurrentBuildFollowsTheBuildsCursorWhenBuildsHasFocus: esc leaves the
// detail open, so l after moving down must not show the old build's log.
func TestCurrentBuildFollowsTheBuildsCursorWhenBuildsHasFocus(t *testing.T) {
	m := goldenModel(80, 24)
	m.detail = ptr(sampleBuild()) // PROJ-BUILD-44
	m.focus = focusBuilds
	m.builds.move(1) // onto #43

	b, ok := m.currentBuild()
	require.True(t, ok)
	require.Equal(t, "PROJ-BUILD-43", b.Key)

	m.focus = focusMain
	b, _ = m.currentBuild()
	require.Equal(t, "PROJ-BUILD-44", b.Key, "Main still acts on the open build")
}

// TestResizeReflowsTheLogViewport: the viewport holds its own dimensions.
func TestResizeReflowsTheLogViewport(t *testing.T) {
	m := logModel()
	require.Equal(t, 80, m.logs.vp.Width)

	m, _ = send(m, windowSize(120, 40))
	require.Equal(t, 120, m.logs.vp.Width)
	require.Equal(t, m.logHeight(), m.logs.vp.Height)
}

func windowSize(w, h int) tea.WindowSizeMsg { return tea.WindowSizeMsg{Width: w, Height: h} }
