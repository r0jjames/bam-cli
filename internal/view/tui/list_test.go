package tui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func words() listState[string] {
	l := newList(func(s string) string { return s })
	l.setItems([]string{"PROJ-BUILD", "PROJ-DEPLOY", "OPS-NIGHTLY", "LAB-SMOKE"})
	return l
}

func TestSelectedFollowsTheCursor(t *testing.T) {
	l := words()
	got, ok := l.selected()
	require.True(t, ok)
	require.Equal(t, "PROJ-BUILD", got)

	l.move(2)
	got, _ = l.selected()
	require.Equal(t, "OPS-NIGHTLY", got)
}

func TestCursorClampsAtBothEnds(t *testing.T) {
	l := words()
	l.move(-5)
	require.Equal(t, 0, l.cursor)
	l.move(99)
	require.Equal(t, 3, l.cursor)
	l.top()
	require.Equal(t, 0, l.cursor)
	l.bottom()
	require.Equal(t, 3, l.cursor)
}

func TestEmptyListHasNoSelection(t *testing.T) {
	l := newList(func(s string) string { return s })
	_, ok := l.selected()
	require.False(t, ok)
	l.move(1)
	require.Equal(t, 0, l.cursor)
}

// TestQueryFiltersCaseInsensitivelyBySubstring is what / does in a panel.
func TestQueryFiltersCaseInsensitivelyBySubstring(t *testing.T) {
	l := words()
	l.setQuery("proj")
	require.Equal(t, []string{"PROJ-BUILD", "PROJ-DEPLOY"}, l.rows())
	got, ok := l.selected()
	require.True(t, ok)
	require.Equal(t, "PROJ-BUILD", got)
}

func TestClearingTheQueryRestoresEveryItem(t *testing.T) {
	l := words()
	l.setQuery("ops")
	require.Len(t, l.rows(), 1)
	l.setQuery("")
	require.Len(t, l.rows(), 4)
}

// TestFilterResetsTheCursorIntoRange: a cursor past the end of a narrowed
// list must not select a row that is no longer shown.
func TestFilterResetsTheCursorIntoRange(t *testing.T) {
	l := words()
	l.bottom()
	require.Equal(t, 3, l.cursor)
	l.setQuery("proj")
	require.Equal(t, 1, l.cursor)
	got, _ := l.selected()
	require.Equal(t, "PROJ-DEPLOY", got)
}

func TestSetItemsKeepsTheCursorInRange(t *testing.T) {
	l := words()
	l.bottom()
	l.setItems([]string{"PROJ-BUILD"})
	require.Equal(t, 0, l.cursor)
}

// TestWindowScrollsWithTheCursor so a long list stays usable in a short panel.
func TestWindowScrollsWithTheCursor(t *testing.T) {
	l := newList(func(s string) string { return s })
	items := make([]string, 20)
	for i := range items {
		items[i] = string(rune('a' + i))
	}
	l.setItems(items)

	start, end := l.window(5)
	require.Equal(t, 0, start)
	require.Equal(t, 5, end)

	l.move(7)
	start, end = l.window(5)
	require.Equal(t, 3, start)
	require.Equal(t, 8, end)

	l.bottom()
	start, end = l.window(5)
	require.Equal(t, 15, start)
	require.Equal(t, 20, end)
}
