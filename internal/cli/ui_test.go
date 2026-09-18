package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/stretchr/testify/require"
)

// TestBareBamOpensTheUIOnATerminal is spec §2's first row.
func TestBareBamOpensTheUIOnATerminal(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run())
	require.Len(t, h.tuiRuns, 1)
	require.Equal(t, "work", h.tuiRuns[0].Initial)
	require.NotEmpty(t, h.tuiRuns[0].Servers)
	require.NotNil(t, h.tuiRuns[0].Connect)
	require.NotNil(t, h.tuiRuns[0].Targets)
}

// TestBareBamOnAPipePrintsHelp keeps the CLI scriptable.
func TestBareBamOnAPipePrintsHelp(t *testing.T) {
	h := newHarness(t)
	h.tty = false
	require.Equal(t, 0, h.run())
	require.Empty(t, h.tuiRuns)
	require.Contains(t, h.stdout.String(), "Usage:")
	require.NotContains(t, h.stdout.String(), "interactive mode arrives in v0.2")
}

// TestDumbTerminalPrintsHelp: TERM=dumb cannot draw the UI.
func TestDumbTerminalPrintsHelp(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	h.vars["TERM"] = "dumb"
	require.Equal(t, 0, h.run())
	require.Empty(t, h.tuiRuns)
	require.Contains(t, h.stdout.String(), "Usage:")
}

func TestNoTUIFlagPrintsHelp(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run("--no-tui"))
	require.Empty(t, h.tuiRuns)
	require.Contains(t, h.stdout.String(), "Usage:")
}

func TestBamNoTUIEnvPrintsHelp(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	h.vars["BAM_NO_TUI"] = "1"
	require.Equal(t, 0, h.run())
	require.Empty(t, h.tuiRuns)
}

// TestBamUIForcesTheUI even though the bare form would too; it is the
// documented way to be explicit.
func TestBamUIForcesTheUI(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run("ui"))
	require.Len(t, h.tuiRuns, 1)
}

// TestBamUIWithoutATerminalIsAConfigError: exit 3, and the message says why.
func TestBamUIWithoutATerminalIsAConfigError(t *testing.T) {
	h := newHarness(t)
	h.tty = false
	require.Equal(t, 3, h.run("ui"))
	require.Empty(t, h.tuiRuns)
	require.Contains(t, strings.ToLower(h.stderr.String()), "terminal")
}

// TestUITargetsClosureReadsTheProjectFile so the Presets panel has content.
func TestUITargetsClosureReadsTheProjectFile(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run())
	targets, err := h.tuiRuns[0].Targets()
	require.NoError(t, err)
	names := make([]string, 0, len(targets))
	for _, tg := range targets {
		names = append(names, tg.Name)
	}
	require.Contains(t, names, "provision-lab")
}

// TestUIConnectClosureIgnoresTheServerFlag: inside the UI, S picks the
// server, so the alias asked for is the alias connected.
func TestUIConnectClosureUsesTheAliasAskedFor(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run())
	svc, err := h.tuiRuns[0].Connect(t.Context(), "work")
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Equal(t, "work", svc.Server.Alias)
}

// TestUITargetsRereadsTheProjectFile: r on the Presets panel is documented as
// picking up an edit made while the UI is open, so the closure cannot serve
// the configuration loaded at start-up.
func TestUITargetsRereadsTheProjectFile(t *testing.T) {
	h := newHarness(t)
	h.tty = true
	require.Equal(t, 0, h.run())
	targets := h.tuiRuns[0].Targets

	first, err := targets()
	require.NoError(t, err)
	require.NotContains(t, targetNames(first), "nightly")

	require.NoError(t, os.WriteFile(filepath.Join(h.root, ".bam.yaml"),
		[]byte(projectYAML+"  nightly:\n    plan: OPS-NIGHTLY\n"), 0o644))

	second, err := targets()
	require.NoError(t, err)
	require.Contains(t, targetNames(second), "nightly", "the edit must be visible without restarting")
}

func targetNames(ts []app.TargetInfo) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}
