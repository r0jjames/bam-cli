package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
