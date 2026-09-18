package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/provider/fake"
	"github.com/stretchr/testify/require"
)

const labOrigin = "https://bamboo.lab.example"

var errBoom = errors.New("bamboo returned 500")

func testService() *app.Service {
	f := &fake.Provider{
		BaseURL:  labOrigin,
		User:     provider.User{Name: "jdoe", FullName: "J Doe"},
		Info:     provider.ServerInfo{Version: "9.6.4"},
		Projects: []provider.Project{{Key: "PROJ", Name: "Example Project", URL: labOrigin + "/browse/PROJ"}},
		Plans: map[string][]provider.Plan{"PROJ": {
			{Key: "PROJ-BUILD", Name: "Build and test", ProjectKey: "PROJ", URL: labOrigin + "/browse/PROJ-BUILD"},
		}},
		Logs: map[string][]string{
			"PROJ-BUILD-INT-44": {
				"12:04:31  [INFO] Running integration suite",
				"12:06:58  [ERROR] ConnectionRefused: bamboo.lab.example:5432",
				"12:06:58  [ERROR] 3 tests failed",
				"12:07:01  Finished with exit code 1",
			},
			"PROJ-BUILD-COMP-44": {"12:03:02  compiled"},
			"PROJ-BUILD-UNIT-44": {"12:03:44  all tests passed"},
		},
		History: map[string][]provider.Build{"PROJ-BUILD": {
			{Key: "PROJ-BUILD-44", PlanKey: "PROJ-BUILD", Number: 44, State: provider.StateFailed,
				URL: labOrigin + "/browse/PROJ-BUILD-44", Duration: 192 * time.Second},
		}},
	}
	return &app.Service{P: f, Cfg: &config.Config{}, Clock: app.SystemClock{}, State: &app.StateStore{}}
}

func TestConnectCmdReportsServerAndUser(t *testing.T) {
	svc := testService()
	d := Deps{Connect: func(context.Context, string) (*app.Service, error) { return svc, nil }}
	msg := connectCmd(d, "lab")()
	got, ok := msg.(connectedMsg)
	require.True(t, ok, "got %T", msg)
	require.Equal(t, "lab", got.Alias)
	require.Equal(t, "9.6.4", got.Info.Version)
	require.Equal(t, "jdoe", got.User.Name)
	require.Same(t, svc, got.Svc)
}

func TestConnectCmdSurfacesFailureAsErrMsg(t *testing.T) {
	boom := errors.New("no token for https://bamboo.lab.example")
	d := Deps{Connect: func(context.Context, string) (*app.Service, error) { return nil, boom }}
	msg := connectCmd(d, "lab")()
	got, ok := msg.(errMsg)
	require.True(t, ok, "got %T", msg)
	require.ErrorIs(t, got.Err, boom)
	require.Equal(t, "connect", got.Where)
}

func TestLoadPlansCmdFlattensEveryProject(t *testing.T) {
	msg := loadPlansCmd(context.Background(), testService(), "")()
	got, ok := msg.(plansLoadedMsg)
	require.True(t, ok, "got %T", msg)
	require.Len(t, got.Plans, 1)
	require.Equal(t, "PROJ-BUILD", got.Plans[0].Key)
}

func TestLoadPlansCmdHonoursTheProjectFilter(t *testing.T) {
	msg := loadPlansCmd(context.Background(), testService(), "OPS")()
	got := msg.(plansLoadedMsg)
	require.Empty(t, got.Plans)
}

func TestLoadBuildsCmdCarriesThePlanKey(t *testing.T) {
	msg := loadBuildsCmd(context.Background(), testService(), "PROJ-BUILD", 25)()
	got, ok := msg.(buildsLoadedMsg)
	require.True(t, ok, "got %T", msg)
	require.Equal(t, "PROJ-BUILD", got.PlanKey)
	require.Len(t, got.Builds, 1)
}

// TestModelStoresWhatItLoads: the messages must land in the panels.
func TestModelStoresWhatItLoads(t *testing.T) {
	m := testModel()
	m, _ = send(m, connectedMsg{Alias: "lab", Svc: testService(),
		Info: provider.ServerInfo{Version: "9.6.4"}, User: provider.User{Name: "jdoe"}})
	require.Equal(t, "9.6.4", m.info.Version)

	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "PROJ-BUILD"}, {Key: "OPS-NIGHTLY"}}})
	require.Equal(t, 2, m.plans.len())

	m, _ = send(m, buildsLoadedMsg{PlanKey: "PROJ-BUILD", Builds: []provider.Build{{Key: "PROJ-BUILD-44"}}})
	require.Equal(t, 1, m.builds.len())
}

// TestErrMsgNeverQuits: a Bamboo error belongs in the status bar, not in an
// exit. Spec §6.
func TestErrMsgNeverQuits(t *testing.T) {
	m, cmd := send(testModel(), errMsg{Err: errBoom, Where: "plans"})
	require.Nil(t, cmd)
	require.Error(t, m.err)
}

// TestEnterOnPlansLoadsBuildsAndMovesFocus is spec §4.1's first row.
func TestEnterOnPlansLoadsBuildsAndMovesFocus(t *testing.T) {
	m := testModel()
	m.svc = testService()
	m, _ = send(m, plansLoadedMsg{Plans: []provider.Plan{{Key: "PROJ-BUILD"}}})
	m, cmd := send(m, mkKey("enter"))
	require.Equal(t, focusBuilds, m.focus)
	require.NotNil(t, cmd)
	got, ok := cmd().(buildsLoadedMsg)
	require.True(t, ok)
	require.Equal(t, "PROJ-BUILD", got.PlanKey)
}
