package tui

import "strings"

// listState is one scrollable, filterable panel list. It holds indexes into
// items rather than copies, so filtering never duplicates a build.
type listState[T any] struct {
	items   []T
	visible []int
	cursor  int
	query   string
	loading bool
	match   func(T) string
}

// newList takes the function that says what text a row is filtered on.
func newList[T any](match func(T) string) listState[T] {
	return listState[T]{match: match}
}

func (l *listState[T]) setItems(items []T) {
	l.items = items
	l.refilter()
}

func (l *listState[T]) setQuery(q string) {
	l.query = q
	l.refilter()
}

func (l *listState[T]) refilter() {
	l.visible = l.visible[:0]
	q := strings.ToLower(l.query)
	for i, it := range l.items {
		if q == "" || strings.Contains(strings.ToLower(l.match(it)), q) {
			l.visible = append(l.visible, i)
		}
	}
	l.clamp()
}

func (l *listState[T]) clamp() {
	if l.cursor >= len(l.visible) {
		l.cursor = len(l.visible) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
}

func (l *listState[T]) move(delta int) {
	l.cursor += delta
	l.clamp()
}

func (l *listState[T]) top() { l.cursor = 0 }

func (l *listState[T]) bottom() {
	l.cursor = len(l.visible) - 1
	l.clamp()
}

func (l listState[T]) len() int { return len(l.visible) }

func (l listState[T]) selected() (T, bool) {
	var zero T
	if l.cursor < 0 || l.cursor >= len(l.visible) {
		return zero, false
	}
	return l.items[l.visible[l.cursor]], true
}

// rows is the filtered items in order, for rendering and for tests.
func (l listState[T]) rows() []T {
	out := make([]T, 0, len(l.visible))
	for _, i := range l.visible {
		out = append(out, l.items[i])
	}
	return out
}

// window is the half-open range of rows to draw in a panel of this height,
// scrolled so the cursor is always inside it.
func (l listState[T]) window(height int) (int, int) {
	if height <= 0 || len(l.visible) == 0 {
		return 0, 0
	}
	if height >= len(l.visible) {
		return 0, len(l.visible)
	}
	start := l.cursor - height + 1
	if start < 0 {
		start = 0
	}
	if maxStart := len(l.visible) - height; start > maxStart {
		start = maxStart
	}
	return start, start + height
}
