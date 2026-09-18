package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

func TestSOpensTheServerPickerWithEveryServer(t *testing.T) {
	m := New(Deps{Servers: []Server{
		{Alias: "lab", URL: "https://bamboo.lab.example"},
		{Alias: "work", URL: "https://bamboo.example.com"},
	}, Initial: "lab"})
	m.width, m.height = 80, 24

	m, _ = send(m, mkKey("S"))
	require.Equal(t, overlayServers, m.overlay)
	require.Equal(t, 2, m.picker.len())
	require.Contains(t, m.View(), "bamboo.lab.example")
}

func TestChoosingAServerReconnectsAndReloads(t *testing.T) {
	var asked []string
	d := Deps{
		Servers: []Server{{Alias: "lab", URL: labOrigin}, {Alias: "work", URL: "https://bamboo.example.com"}},
		Initial: "lab",
		Connect: func(_ context.Context, alias string) (*app.Service, error) {
			asked = append(asked, alias)
			return testService(), nil
		},
	}
	m := New(d)
	m.width, m.height = 80, 24

	m, _ = send(m, mkKey("S"))
	m, _ = send(m, mkKey("j"))
	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, overlayNone, m.overlay, "choosing closes the overlay")
	require.NotNil(t, cmd)
	require.IsType(t, connectedMsg{}, cmd())
	require.Equal(t, []string{"work"}, asked)
}

// TestSwitchingServerCancelsTheWatch: the old server's build is not this
// server's build.
func TestSwitchingServerCancelsTheWatch(t *testing.T) {
	m := New(Deps{Servers: []Server{{Alias: "lab"}, {Alias: "work"}}, Initial: "lab",
		Connect: func(context.Context, string) (*app.Service, error) { return testService(), nil }})
	m.width, m.height = 80, 24
	cancelled := false
	m.watchCancel = func() { cancelled = true }
	m.detail = ptr(sampleBuild())

	m, _ = send(m, mkKey("S"))
	m, _ = send(m, mkKey("j"))
	m, _ = send(m, mkKey("enter"))
	require.True(t, cancelled)
	require.Nil(t, m.detail, "the old server's build detail is cleared")
}

func TestEscClosesThePickerWithoutSwitching(t *testing.T) {
	m := New(Deps{Servers: []Server{{Alias: "lab"}, {Alias: "work"}}, Initial: "lab"})
	m.width, m.height = 80, 24
	m, _ = send(m, mkKey("S"))
	m, _ = send(m, mkKey("esc"))
	require.Equal(t, overlayNone, m.overlay)
	require.Equal(t, "lab", m.server)
}

func TestOverlayStaysInsideTheTerminal(t *testing.T) {
	m := goldenModel(80, 24)
	m.overlay = overlayServers
	m.picker.setItems([]pickerItem{{Label: "lab", Detail: labOrigin}})
	for _, line := range strings.Split(m.View(), "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}
}

// TestPickerKeysDoNotLeakToThePanels: while an overlay is open, j moves the
// picker and nothing else.
func TestPickerKeysDoNotLeakToThePanels(t *testing.T) {
	m := goldenModel(80, 24)
	m.deps.Servers = append(m.deps.Servers, Server{Alias: "work", URL: "https://bamboo.example.com"})
	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "A"}, {Key: "B"}}})
	m, _ = send(m, mkKey("S"))
	before := m.plans.cursor
	m, _ = send(m, mkKey("j"))
	require.Equal(t, before, m.plans.cursor)
	require.Equal(t, 1, m.picker.cursor)
}

func TestPOpensTheProjectPickerWithAnAllRow(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, cmd := send(m, mkKey("P"))
	require.Equal(t, overlayProjects, m.overlay)
	require.NotNil(t, cmd, "the picker loads the project list")

	m, _ = send(m, cmd().(projectsLoadedMsg))
	rows := m.picker.rows()
	require.Equal(t, "all projects", rows[0].Label)
	require.Equal(t, "", rows[0].Value)
	require.Equal(t, "PROJ", rows[1].Value)
}

func TestChoosingAProjectReloadsThePlans(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, mkKey("P"))
	m, _ = send(m, projectsLoadedMsg{Projects: []provider.Project{{Key: "PROJ"}, {Key: "OPS"}}})
	m, _ = send(m, mkKey("j")) // onto PROJ
	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, "PROJ", m.project)
	require.Equal(t, overlayNone, m.overlay)
	require.NotNil(t, cmd)
	require.IsType(t, plansLoadedMsg{}, cmd())
}

func TestChoosingAllProjectsClearsTheFilter(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.project = "PROJ"
	m, _ = send(m, mkKey("P"))
	m, _ = send(m, projectsLoadedMsg{Projects: []provider.Project{{Key: "PROJ"}}})
	m.picker.top()
	m, _ = send(m, mkKey("enter"))
	require.Equal(t, "", m.project)
}

func TestBOpensTheBranchPickerForTheSelectedPlan(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "PROJ-PROV"}}})
	m, cmd := send(m, mkKey("b"))
	require.Equal(t, overlayBranches, m.overlay)
	require.NotNil(t, cmd)

	m, _ = send(m, branchesLoadedMsg{MasterKey: "PROJ-PROV", Branches: []provider.Branch{
		{Key: "PROJ-PROV12", ShortName: "develop", PlanKey: "PROJ-PROV"},
	}})
	rows := m.picker.rows()
	require.Equal(t, "default branch", rows[0].Label)
	require.Equal(t, "PROJ-PROV", rows[0].Value)
	require.Equal(t, "develop", rows[1].Label)
	require.Equal(t, "PROJ-PROV12", rows[1].Value)
}

func TestChoosingABranchLoadsThatBranchPlansBuilds(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "PROJ-PROV"}}})
	m, _ = send(m, mkKey("b"))
	m, _ = send(m, branchesLoadedMsg{MasterKey: "PROJ-PROV", Branches: []provider.Branch{
		{Key: "PROJ-PROV12", ShortName: "develop", PlanKey: "PROJ-PROV"},
	}})
	m, _ = send(m, mkKey("j"))
	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, overlayNone, m.overlay)
	require.Equal(t, "PROJ-PROV12", m.buildsPlan)
	require.NotNil(t, cmd)
	require.Equal(t, "PROJ-PROV12", cmd().(buildsLoadedMsg).PlanKey)
}

// TestBranchSwitchCancelsTheWatch: the watched build belongs to the old
// branch plan.
func TestBranchSwitchCancelsTheWatch(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "PROJ-PROV"}}})
	cancelled := false
	m.watchCancel = func() { cancelled = true }
	m.detail = ptr(sampleBuild())
	m, _ = send(m, mkKey("b"))
	m, _ = send(m, branchesLoadedMsg{MasterKey: "PROJ-PROV", Branches: nil})
	m, _ = send(m, mkKey("enter"))
	require.True(t, cancelled)
	require.Nil(t, m.detail)
}

func TestBWithNoPlanSelectedDoesNothing(t *testing.T) {
	m := goldenModel(80, 24)
	m.svc = testService()
	m.plans.setItems(nil)
	m, cmd := send(m, mkKey("b"))
	require.Equal(t, overlayNone, m.overlay)
	require.Nil(t, cmd)
}
