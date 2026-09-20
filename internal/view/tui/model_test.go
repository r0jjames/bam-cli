package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

func testModel() Model {
	m := New(Deps{Servers: []Server{{Alias: "lab", URL: "https://bamboo.lab.example"}}, Initial: "lab"})
	m.width, m.height = 80, 24
	return m
}

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// TestTabCyclesPanels pins the tab order of spec §3.
func TestTabCyclesPanels(t *testing.T) {
	m := testModel()
	require.Equal(t, focusPlans, m.focus)
	for _, want := range []focus{focusBuilds, focusPresets, focusMain, focusPlans} {
		m, _ = send(m, mkKey("tab"))
		require.Equal(t, want, m.focus)
	}
}

// TestNumberKeysFocusPanelsDirectly pins 1, 2 and 3.
func TestNumberKeysFocusPanelsDirectly(t *testing.T) {
	m := testModel()
	for _, tc := range []struct {
		key  string
		want focus
	}{{"2", focusBuilds}, {"3", focusPresets}, {"1", focusPlans}} {
		m, _ = send(m, mkKey(tc.key))
		require.Equal(t, tc.want, m.focus, "key %q", tc.key)
	}
}

// TestBackResolvesInSpecOrder pins spec §4.1: overlay, then the log screen,
// then focus, then quit. Each level must be popped before the next one is.
func TestBackResolvesInSpecOrder(t *testing.T) {
	m := testModel()
	m.overlay, m.screen, m.focus = overlayHelp, screenLogs, focusMain

	m, cmd := send(m, mkKey("esc"))
	require.Equal(t, overlayNone, m.overlay)
	require.Equal(t, screenLogs, m.screen, "the overlay closes before the log screen does")
	require.Nil(t, cmd)

	m, _ = send(m, mkKey("esc"))
	require.Equal(t, screenColumns, m.screen)
	require.Equal(t, focusMain, m.focus, "the log screen closes before focus moves")

	m, _ = send(m, mkKey("esc"))
	require.Equal(t, focusBuilds, m.focus, "main's parent is Builds, not Presets")

	m, _ = send(m, mkKey("esc"))
	require.Equal(t, focusPlans, m.focus)

	_, cmd = send(m, mkKey("esc"))
	require.NotNil(t, cmd, "esc on Plans quits")
	require.IsType(t, tea.QuitMsg{}, cmd())
}

// TestPresetsBacksOutToPlans keeps Presets a sibling of Builds, not its child.
func TestPresetsBacksOutToPlans(t *testing.T) {
	m := testModel()
	m.focus = focusPresets
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, focusPlans, m.focus)
}

// TestQuitKeys pins q and ctrl-c from the columns screen.
func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := send(testModel(), mkKey(k))
		require.NotNil(t, cmd, "key %q", k)
		require.IsType(t, tea.QuitMsg{}, cmd(), "key %q", k)
	}
}

// TestWindowSizeIsRemembered so the layout can size its panels.
func TestWindowSizeIsRemembered(t *testing.T) {
	m, _ := send(testModel(), tea.WindowSizeMsg{Width: 120, Height: 40})
	require.Equal(t, 120, m.width)
	require.Equal(t, 40, m.height)
}

// TestMovementAppliesToTheFocusedPanel: j and k must not move two cursors.
func TestMovementAppliesToTheFocusedPanel(t *testing.T) {
	m := testModel()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{{Key: "A"}, {Key: "B"}}})
	m, _ = send(m, buildsLoadedMsg{Gen: m.buildsGen, PlanKey: "A", Builds: []provider.Build{{Key: "A-1"}, {Key: "A-2"}}})

	m, _ = send(m, mkKey("j"))
	require.Equal(t, 1, m.plans.cursor)
	require.Equal(t, 0, m.builds.cursor)

	m.focus = focusBuilds
	m, _ = send(m, mkKey("j"))
	require.Equal(t, 1, m.plans.cursor, "the plans cursor must not move")
	require.Equal(t, 1, m.builds.cursor)
}

func TestGAndShiftGJumpToTheEnds(t *testing.T) {
	m := testModel()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{{Key: "A"}, {Key: "B"}, {Key: "C"}}})
	m, _ = send(m, mkKey("G"))
	require.Equal(t, 2, m.plans.cursor)
	m, _ = send(m, mkKey("g"))
	require.Equal(t, 0, m.plans.cursor)
}

// TestEnterOnBuildsOpensDetailAndFocusesMain is spec §4.1's second row.
func TestEnterOnBuildsOpensDetailAndFocusesMain(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m, _ = send(m, buildsLoadedMsg{Gen: m.buildsGen, PlanKey: "PROJ-BUILD", Builds: []provider.Build{sampleBuild()}})
	m.focus = focusBuilds

	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, focusMain, m.focus)
	require.NotNil(t, m.detail)
	require.Equal(t, "PROJ-BUILD-44", m.detail.Key)
	require.True(t, m.expanded["Test"], "the failed stage opens")
	require.NotNil(t, cmd, "opening a build starts its watch")
}

func TestEnterOnAStageTogglesIt(t *testing.T) {
	m := testModel()
	m.detail, m.expanded, m.focus = ptr(sampleBuild()), map[string]bool{}, focusMain
	require.Len(t, m.treeRows(), 3)

	m, _ = send(m, mkKey("enter")) // the cursor is on Build
	require.True(t, m.expanded["Build"])
	require.Len(t, m.treeRows(), 5)

	m, _ = send(m, mkKey("enter"))
	require.False(t, m.expanded["Build"])
	require.Len(t, m.treeRows(), 3)
}

// TestTreeCursorClampsWhenAStageCollapses: collapsing above the cursor must
// not leave it pointing past the end.
func TestTreeCursorClampsWhenAStageCollapses(t *testing.T) {
	m := testModel()
	m.detail, m.expanded, m.focus = ptr(sampleBuild()), map[string]bool{"Build": true}, focusMain
	m.treeCursor = 4 // the last row while Build is open
	m, _ = send(m, mkKey("enter"))
	require.Less(t, m.treeCursor, len(m.treeRows()))
}

// TestEnterOnAPresetSelectsItsPlan, spec §4.1's last row. A preset is a
// shortcut to a plan, so it moves the Plans cursor and loads that plan's
// builds. It does not run anything: part A is read-only.
func TestEnterOnAPresetSelectsItsPlan(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{{Key: "PROJ-BUILD"}, {Key: "PROJ-PROV"}}})
	// The panel's own data is what resolves now, so it carries the branch
	// exactly as app.DescribeTargets reports it.
	m, _ = send(m, presetsLoadedMsg{Targets: []app.TargetInfo{{Name: "smoke", Plan: "PROJ-PROV", Branch: "develop"}}})
	m.focus = focusPresets

	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, focusBuilds, m.focus)
	sel, ok := m.plans.selected()
	require.True(t, ok)
	require.Equal(t, "PROJ-PROV", sel.Key, "the Plans cursor lands on the master plan")
	require.NotNil(t, cmd)
	// smoke names branch develop, so its builds come from the branch plan.
	require.Equal(t, "PROJ-PROV12", cmd().(buildsLoadedMsg).PlanKey)
}

// TestEnterOnAPresetWhosePlanIsNotListedStillLoadsIt: a preset may name a
// plan in a project the current filter hides.
func TestEnterOnAPresetWhosePlanIsNotListedStillLoadsIt(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m, _ = send(m, presetsLoadedMsg{Targets: []app.TargetInfo{{Name: "build", Plan: "PROJ-BUILD"}}})
	m.focus = focusPresets
	m, cmd := send(m, mkKey("enter"))
	require.NotNil(t, cmd)
	require.Equal(t, "PROJ-BUILD", cmd().(buildsLoadedMsg).PlanKey)
	require.Equal(t, focusBuilds, m.focus)
}

// TestNoLowercaseKeyMutatesAnything is part B's rule in place of part A's
// "nothing mutates": R only opens a form, and the keys that reach the server
// are uppercase or chorded, so neither is one keystroke away from a
// forty-minute pipeline.
func TestNoLowercaseKeyMutatesAnything(t *testing.T) {
	mutating := []key.Binding{keys.Run, keys.Trigger, keys.Cancel}
	for _, b := range mutating {
		for _, k := range b.Keys() {
			require.True(t, strings.HasPrefix(k, "ctrl+") || strings.ToLower(k) != k,
				"key %q reaches a mutation and is lowercase", k)
		}
	}
	// R opens a form and sends nothing, so it is the one of the three that
	// needs no confirmation. ctrl-R cannot be struck while a field is being
	// edited, and C always asks.
	require.Equal(t, []string{"R"}, keys.Run.Keys())
	require.Equal(t, []string{"ctrl+r"}, keys.Trigger.Keys())
	require.Equal(t, []string{"C"}, keys.Cancel.Keys())
}

// TestRefreshReloadsTheFocusedPanelOnly, spec §5.
func TestRefreshReloadsTheFocusedPanelOnly(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.buildsPlan = "PROJ-BUILD"

	m.focus = focusPlans
	_, cmd := send(m, mkKey("r"))
	require.NotNil(t, cmd)
	require.IsType(t, plansLoadedMsg{}, cmd())

	m.focus = focusBuilds
	_, cmd = send(m, mkKey("r"))
	require.NotNil(t, cmd)
	require.Equal(t, "PROJ-BUILD", cmd().(buildsLoadedMsg).PlanKey)
}

// TestRefreshOnPresetsRereadsTheProjectFile, so editing .bam.yaml in another
// window shows up without restarting the UI.
func TestRefreshOnPresetsRereadsTheProjectFile(t *testing.T) {
	calls := 0
	m := New(Deps{Targets: func() ([]app.TargetInfo, error) {
		calls++
		return []app.TargetInfo{{Name: "smoke", Plan: "LAB-SMOKE"}}, nil
	}})
	m.width, m.height = 80, 24
	m.focus = focusPresets

	_, cmd := send(m, mkKey("r"))
	require.NotNil(t, cmd)
	require.IsType(t, presetsLoadedMsg{}, cmd())
	require.Equal(t, 1, calls)
}

func TestRefreshOnMainRefetchesTheBuild(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.focus = focusMain
	m.detail = ptr(provider.Build{Key: "PROJ-BUILD-44", PlanKey: "PROJ-BUILD"})

	_, cmd := send(m, mkKey("r"))
	require.NotNil(t, cmd)
	require.IsType(t, buildLoadedMsg{}, cmd())
}

// TestRefreshClearsTheError so r is the documented retry after EventError.
func TestRefreshClearsTheError(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m.err = errBoom
	m, _ = send(m, mkKey("r"))
	require.NoError(t, m.err)
}

func TestRefreshWithoutAConnectionDoesNothing(t *testing.T) {
	m := testModel()
	m.svc = nil
	_, cmd := send(m, mkKey("r"))
	require.Nil(t, cmd)
}

func TestBuildLoadedMsgUpdatesTheDetailWithoutLosingTheTreeCursor(t *testing.T) {
	m := testModel()
	m.detail, m.expanded, m.treeCursor = ptr(sampleBuild()), map[string]bool{"Test": true}, 2
	m, _ = send(m, buildLoadedMsg{Build: sampleBuild()})
	require.Equal(t, 2, m.treeCursor)
	require.True(t, m.expanded["Test"], "refreshing must not collapse what the user opened")
}

// TestPresetRowShowsNamePlanAndBranch, spec §7. Two presets on one plan are
// otherwise indistinguishable.
func TestPresetRowShowsNamePlanAndBranch(t *testing.T) {
	m := goldenModel(120, 40)
	m, _ = send(m, presetsLoadedMsg{Targets: []app.TargetInfo{
		{Name: "smoke", Plan: "PROJ-PROV", Branch: "develop"},
	}})
	rows := m.presetRows(40, 4)
	require.Contains(t, rows, "smoke")
	require.Contains(t, rows, "PROJ-PROV")
	require.Contains(t, rows, "develop")
}

// TestInitAfterHandshakeFillsPlans pins the generation Init hands to its
// plans request. Run connects before the program loop, so Init takes the
// already-connected branch; Init's receiver is a copy, so a bump made there
// is lost and every plansLoadedMsg it asked for would be dropped as stale.
func TestInitAfterHandshakeFillsPlans(t *testing.T) {
	d := Deps{
		Connect: func(context.Context, string) (*app.Service, error) { return testService(), nil },
		Servers: []Server{{Alias: "lab", URL: "https://bamboo.lab.example"}},
		Initial: "lab",
	}
	m := New(d).withContext(t.Context())
	msg, ok := connectCmd(t.Context(), d, d.Initial, 0)().(connectedMsg)
	require.True(t, ok)
	m = m.connected(msg)

	// Run what Init asks for and feed every message back, the way the
	// bubbletea loop does.
	var deliver func(Model, tea.Cmd) Model
	deliver = func(m Model, cmd tea.Cmd) Model {
		if cmd == nil {
			return m
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, sub := range msg {
				m = deliver(m, sub)
			}
		case nil:
		default:
			m, cmd = send(m, msg)
			m = deliver(m, cmd)
		}
		return m
	}
	m = deliver(m, m.Init())

	require.NoError(t, m.err)
	require.NotZero(t, m.plans.len(), "the Plans panel is empty after start-up")
	require.False(t, m.plans.loading)
}
