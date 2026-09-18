package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// mkKey builds the tea.KeyMsg a terminal would send for s.
func mkKey(s string) tea.KeyMsg {
	switch s {
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestEveryBindingAppearsInHelp keeps the help overlay honest: the binding
// set is the only source of the table, so a new key cannot ship undocumented.
func TestEveryBindingAppearsInHelp(t *testing.T) {
	documented := map[string]bool{}
	for _, row := range keys.helpRows() {
		documented[row.Keys] = true
	}
	for _, b := range allBindings(keys) {
		require.True(t, documented[b.Help().Key], "binding %q is missing from helpRows()", b.Help().Key)
	}
}

// TestNoKeyIsBoundTwice catches a key silently shadowing another. h and l are
// checked by name because spec §4 frees them from movement on purpose.
func TestNoKeyIsBoundTwice(t *testing.T) {
	seen := map[string]string{}
	for _, b := range allBindings(keys) {
		for _, k := range b.Keys() {
			prev, dup := seen[k]
			require.False(t, dup, "key %q is bound to both %q and %q", k, prev, b.Help().Desc)
			seen[k] = b.Help().Desc
		}
	}
	require.NotContains(t, seen, "h", "h must stay free: the left column is vertical only")
	require.Equal(t, "logs", seen["l"])
}

// TestQuitBindings pins the two ways out.
func TestQuitBindings(t *testing.T) {
	require.True(t, key.Matches(mkKey("q"), keys.Quit))
	require.True(t, key.Matches(mkKey("ctrl+c"), keys.Quit))
}
