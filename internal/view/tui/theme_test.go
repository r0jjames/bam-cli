package tui

import (
	"testing"

	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/view/style"
	"github.com/stretchr/testify/require"
)

// TestStateStyleCoversEveryState keeps style the single definition: every
// state the provider can report must have a colour here and a glyph there.
func TestStateStyleCoversEveryState(t *testing.T) {
	for _, s := range provider.AllStates() {
		require.NotEmpty(t, stateStyle(s).GetForeground(), "no colour for %s", s)
		require.Equal(t, style.Glyph(s), stateGlyph(s), "glyph for %s must come from style", s)
	}
}

// TestStateCellReadsAsGlyphPlusLabel guards the rule that a state is never
// signalled by colour alone.
func TestStateCellReadsAsGlyphPlusLabel(t *testing.T) {
	cell := stateCell(provider.StateFailed)
	require.Contains(t, cell, style.Glyph(provider.StateFailed))
	require.Contains(t, cell, "failed")
}

// TestUnknownStateFallsBack mirrors style.Glyph's own fallback.
func TestUnknownStateFallsBack(t *testing.T) {
	require.Equal(t, style.Glyph(provider.StateUnknown), stateGlyph(provider.State("nonsense")))
}
