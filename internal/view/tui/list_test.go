package tui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetOrderSortsTheVisibleRows(t *testing.T) {
	l := newList(func(s string) string { return s })
	l.setItems([]string{"b", "c", "a"})
	l.setOrder(func(a, b string) bool { return a < b })
	require.Equal(t, []string{"a", "b", "c"}, l.rows())

	l.setQuery("b")
	require.Equal(t, []string{"b"}, l.rows(), "filter and order compose")
}

func TestSelectFirstMovesTheCursorOntoAMatch(t *testing.T) {
	l := newList(func(s string) string { return s })
	l.setItems([]string{"a", "b", "c"})
	require.True(t, l.selectFirst(func(s string) bool { return s == "c" }))
	got, _ := l.selected()
	require.Equal(t, "c", got)
	require.False(t, l.selectFirst(func(s string) bool { return s == "z" }))
	got, _ = l.selected()
	require.Equal(t, "c", got, "no match leaves the cursor where it was")
}
