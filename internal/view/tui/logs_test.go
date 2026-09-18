package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

func logLines() []string {
	return []string{
		"12:04:31  [INFO] Running integration suite",
		"12:06:58  [ERROR] ConnectionRefused: bamboo.lab.example:5432",
		"12:06:58  [ERROR] 3 tests failed",
		"12:07:01  Finished with exit code 1",
	}
}

func logModel() Model {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.detail = ptr(sampleBuild())
	m, _ = send(m, logsLoadedMsg{JobKey: "PROJ-BUILD-INT-44", Title: "PROJ-BUILD-INT-44",
		URL: labOrigin + "/browse/PROJ-BUILD-INT-44", Lines: logLines()})
	m.screen = screenLogs
	return m
}

// TestLOpensTheLogScreenFullWidth, spec §3.1.
func TestLOpensTheLogScreenFullWidth(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.detail = ptr(sampleBuild())
	m.focus = focusBuilds
	m, cmd := send(m, mkKey("l"))
	require.Equal(t, screenLogs, m.screen)
	require.NotNil(t, cmd)

	m, _ = send(m, logsLoadedMsg{JobKey: "PROJ-BUILD-INT-44", Title: "PROJ-BUILD-INT-44", Lines: logLines()})
	v := m.View()
	require.NotContains(t, v, "1 Plans", "the left column is hidden")
	require.Contains(t, v, "ConnectionRefused")
}

// TestLDefaultsToTheFailedJobs, matching bam logs --last --failed.
func TestLDefaultsToTheFailedJobs(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.detail = ptr(sampleBuild())
	m.focus = focusBuilds
	_, cmd := send(m, mkKey("l"))
	msg, ok := cmd().(logsLoadedMsg)
	require.True(t, ok, "got %T", cmd())
	require.False(t, msg.All)
	require.Equal(t, "PROJ-BUILD-INT-44", msg.JobKey, "the failed job, not the first job")
}

func TestAOpensEveryJobsLog(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.detail = ptr(sampleBuild())
	m.focus = focusBuilds
	_, cmd := send(m, mkKey("a"))
	msg, ok := cmd().(logsLoadedMsg)
	require.True(t, ok, "got %T", cmd())
	require.True(t, msg.All)
}

func TestEscLeavesTheLogScreen(t *testing.T) {
	m := logModel()
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, screenColumns, m.screen)
}

// TestLogHeaderNamesTheJobAndThePosition, spec §3.1's header.
func TestLogHeaderNamesTheJobAndThePosition(t *testing.T) {
	v := logModel().View()
	require.Contains(t, v, "PROJ-BUILD-INT-44")
	require.Contains(t, v, "logs")
	require.Contains(t, v, "failed")
	require.Contains(t, v, "/4", "the line count is in the header")
}

func TestLogScreenNeverExceedsTheTerminal(t *testing.T) {
	for i, line := range strings.Split(logModel().View(), "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80, "line %d", i)
	}
}

func TestLogGolden(t *testing.T) {
	requireGolden(t, "logs-80x24", logModel().View())
}

func TestOpeningLogsWithNoBuildDoesNothing(t *testing.T) {
	m := goldenModel(80, 24)
	m.detail = nil
	m.builds.setItems([]provider.Build{})
	m, cmd := send(m, mkKey("l"))
	require.Equal(t, screenColumns, m.screen)
	require.Nil(t, cmd)
}

func TestSearchJumpsToTheFirstMatch(t *testing.T) {
	m := logModel()
	m.logs.search("connectionrefused")
	require.Equal(t, []int{1}, m.logs.matches, "search is case-insensitive")
	require.Equal(t, 1, m.logs.line)
}

func TestNAndShiftNWalkTheMatchesAndWrap(t *testing.T) {
	m := logModel()
	m.logs.search("ERROR")
	require.Equal(t, []int{1, 2}, m.logs.matches)
	require.Equal(t, 0, m.logs.match)

	m.logs.nextMatch(1)
	require.Equal(t, 1, m.logs.match)
	m.logs.nextMatch(1)
	require.Equal(t, 0, m.logs.match, "n wraps at the end")
	m.logs.nextMatch(-1)
	require.Equal(t, 1, m.logs.match, "N wraps at the start")
}

// TestNWithNoSearchGoesToTheNextFailure, which is what the key line promises.
func TestNWithNoSearchGoesToTheNextFailure(t *testing.T) {
	m := logModel()
	m.logs.query = ""
	m.logs.nextFailure(1)
	require.Equal(t, 1, m.logs.line, "the first ERROR line")
}

func TestSlashOpensTheSearchInputAndKeysGoToIt(t *testing.T) {
	m := logModel()
	m, _ = send(m, mkKey("/"))
	require.Equal(t, inputSearch, m.inputFor)

	for _, r := range "quota" {
		m, _ = send(m, mkKey(string(r)))
	}
	require.Equal(t, "quota", m.input.Value())
	require.Equal(t, screenLogs, m.screen, "q while typing does not quit")

	m, _ = send(m, mkKey("enter"))
	require.Equal(t, inputNone, m.inputFor)
	require.Equal(t, "quota", m.logs.query)
}

func TestEscCancelsTheSearchInputBeforeTheScreen(t *testing.T) {
	m := logModel()
	m, _ = send(m, mkKey("/"))
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, inputNone, m.inputFor)
	require.Equal(t, screenLogs, m.screen, "the input closes before the screen")
}

func TestSlashInAPanelFiltersThatPanel(t *testing.T) {
	m := goldenModel(80, 24)
	m, _ = send(m, mkKey("/"))
	require.Equal(t, inputFilter, m.inputFor)
	for _, r := range "ops" {
		m, _ = send(m, mkKey(string(r)))
	}
	m, _ = send(m, mkKey("enter"))
	require.Equal(t, 1, m.plans.len())
	sel, _ := m.plans.selected()
	require.Equal(t, "OPS-NIGHTLY", sel.Key)
}
