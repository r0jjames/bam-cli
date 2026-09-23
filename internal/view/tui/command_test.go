package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func typeLine(m Model, s string) Model {
	for _, r := range s {
		m, _ = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func runCmdLine(m Model, line string) (Model, tea.Cmd) {
	m, _ = send(m, mkKey(":"))
	m = typeLine(m, line)
	return send(m, mkKey("enter"))
}

func TestColonOpensThePromptOnHomeAndPanels(t *testing.T) {
	m, _ := send(homeModel(), mkKey(":"))
	require.Equal(t, inputCommand, m.inputFor)
	p := homeModel()
	p.screen = screenColumns
	p, _ = send(p, mkKey(":"))
	require.Equal(t, inputCommand, p.inputFor)
}

func TestColonIsTextInsideAnotherInput(t *testing.T) {
	m, _ := send(homeModel(), mkKey("/"))
	m, _ = send(m, mkKey(":"))
	require.Equal(t, inputFilter, m.inputFor)
	require.Equal(t, ":", m.input.Value())
}

func TestPresetsAndPlansCommands(t *testing.T) {
	m, _ := runCmdLine(homeModel(), "presets")
	require.Equal(t, homePresets, m.home.view)
	m, _ = runCmdLine(m, "plans")
	require.Equal(t, homePlans, m.home.view)

	p := homeModel()
	p.screen = screenColumns
	p, _ = runCmdLine(p, "plans")
	require.Equal(t, screenHome, p.screen, ":plans from the panels returns Home")
}

func TestAUniquePrefixIsAccepted(t *testing.T) {
	m, _ := runCmdLine(homeModel(), "pre")
	require.Equal(t, homePresets, m.home.view)
}

func TestAnAmbiguousOrUnknownCommandIsAStatusError(t *testing.T) {
	m, _ := runCmdLine(homeModel(), "p")
	require.EqualError(t, m.err, `ambiguous command "p": plans, presets, project`)
	require.Equal(t, homePlans, m.home.view)

	m, _ = runCmdLine(homeModel(), "frobnicate")
	require.EqualError(t, m.err, `unknown command "frobnicate"; commands: plans presets project server sort quit`)
	require.Equal(t, inputNone, m.inputFor)
}

func TestSortCommand(t *testing.T) {
	m, _ := runCmdLine(homeModel(), "sort -age")
	require.Equal(t, homeSort{col: colAge, desc: true}, m.home.sort)
	m, _ = runCmdLine(m, "sort bogus")
	require.EqualError(t, m.err, `unknown sort column "bogus"; columns: key name state age`)
	m, _ = runCmdLine(homeModel(), "sort")
	require.EqualError(t, m.err, ":sort needs a column")
}

func TestProjectCommand(t *testing.T) {
	m, cmd := runCmdLine(homeModel(), "project OPS")
	require.Equal(t, "OPS", m.project)
	require.NotNil(t, cmd)
	m, _ = runCmdLine(m, "project all")
	require.Equal(t, "", m.project)

	// finding 4: rejecting an unknown project only makes sense once projects
	// are actually configured; projectChoices falls back to the loaded
	// plans' projects otherwise, and those are not a validation list.
	configured := homeModel()
	configured.svc.Cfg.Project = &config.ProjectFile{Version: 1, Projects: []string{"OPS", "PROJ"}}
	m, _ = runCmdLine(configured, "project NOPE")
	require.EqualError(t, m.err, `project "NOPE" is not listed; projects: OPS PROJ`)
}

// TestProjectCommandAcceptsAnyKeyWhenNothingIsConfigured covers finding 4:
// with no configured projects, :project must accept any key rather than
// reject it against the projects merely seen in the loaded plans.
func TestProjectCommandAcceptsAnyKeyWhenNothingIsConfigured(t *testing.T) {
	m, cmd := runCmdLine(homeModel(), "project OTHER")
	require.Equal(t, "OTHER", m.project)
	require.NoError(t, m.err)
	require.NotNil(t, cmd)
}

func TestServerCommand(t *testing.T) {
	m := homeModel()
	m.deps.Servers = append(m.deps.Servers, Server{Alias: "work", URL: "https://bamboo.example.com"})
	m, _ = runCmdLine(m, "server work")
	require.Equal(t, "work", m.server)
	m, _ = runCmdLine(m, "server nope")
	require.EqualError(t, m.err, `no server "nope"; servers: lab work`)
}

func TestQuitCommand(t *testing.T) {
	_, cmd := runCmdLine(homeModel(), "q")
	require.NotNil(t, cmd)
	require.IsType(t, tea.QuitMsg{}, cmd())
}

func TestCompletion(t *testing.T) {
	m := homeModel()
	require.Equal(t, []string{"plans", "presets", "project"}, m.complete("p"))
	require.Equal(t, []string{"project OPS", "project PROJ", "project all"}, m.complete("project "))
	require.Equal(t, []string{"sort age"}, m.complete("sort ag"))
	require.Equal(t, []string{"sort -age"}, m.complete("sort -a"))
}

func TestTabCompletesAUniqueCandidate(t *testing.T) {
	m, _ := send(homeModel(), mkKey(":"))
	m = typeLine(m, "pres")
	m, _ = send(m, mkKey("tab"))
	require.Equal(t, "presets", m.input.Value())
	m, _ = send(homeModel(), mkKey(":"))
	m = typeLine(m, "pro")
	m, _ = send(m, mkKey("tab"))
	require.Equal(t, "project ", m.input.Value(), "a command with an argument gets its space")
}

func TestHelpListsTheCommands(t *testing.T) {
	m := goldenModel(120, 44)
	m, _ = send(m, mkKey("?"))
	v := m.View()
	for _, r := range commandHelpRows() {
		require.Contains(t, v, r.Keys)
	}
}
