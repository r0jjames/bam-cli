package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

func plan(project, key string) provider.Plan {
	return provider.Plan{Key: key, Name: key, ProjectKey: project}
}

func TestPlansLoadedKeepsTheCursorOnItsPlan(t *testing.T) {
	m := testModel()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{plan("PROJ", "PROJ-A"), plan("PROJ", "PROJ-B")}})
	m.plans.cursor = 1 // PROJ-B
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{
		plan("PROJ", "PROJ-0"), plan("PROJ", "PROJ-A"), plan("PROJ", "PROJ-B")}})
	got, _ := m.plans.selected()
	require.Equal(t, "PROJ-B", got.Key, "a new row above must not move the cursor off its plan")
}

func TestPlansLoadedKeepsTheRowsOfAFailedProject(t *testing.T) {
	m := testModel()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{plan("OPS", "OPS-N"), plan("PROJ", "PROJ-A")}})
	require.False(t, m.home.stale)
	require.Equal(t, now, m.home.loadedAt)

	boom := errors.New("bamboo returned 500")
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Plans: []provider.Plan{plan("PROJ", "PROJ-B")},
		Failed: []string{"OPS"}, Err: boom})
	keys := []string{}
	for _, p := range m.plans.rows() {
		keys = append(keys, p.Key)
	}
	require.Equal(t, []string{"OPS-N", "PROJ-B"}, keys, "OPS keeps its old row; PROJ updates")
	require.ErrorIs(t, m.err, boom)
	require.True(t, m.home.stale)
	require.Equal(t, now, m.home.loadedAt, "a partial load is not a fresh one")
}

func TestPlansLoadedNamesAMissingProject(t *testing.T) {
	m := testModel()
	m, _ = send(m, plansLoadedMsg{Gen: m.plansGen, Missing: []string{"GONE"}})
	require.Equal(t, "project GONE is configured but not on lab", m.status)
}

func TestAPlansErrorMarksHomeStale(t *testing.T) {
	m := testModel()
	m, _ = send(m, errMsg{Err: errors.New("down"), Stream: streamPlans, Gen: m.plansGen})
	require.True(t, m.home.stale)
}
