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
