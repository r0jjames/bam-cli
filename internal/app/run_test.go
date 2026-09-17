package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTriggersChangedVariablesAndRecordsLast(t *testing.T) {
	p := fakeBamboo()
	p.TriggerResult = provider.Build{Key: "PROJ-PROV12-10", Number: 10, State: provider.StateQueued}
	s := newService(t, p)
	withEnv(s, map[string]string{"LAB_DB_PASSWORD": "hunter2"})
	ref, _ := s.ResolvePlan(bg, "provision-lab", "")
	vs, err := s.ResolveVars(bg, ref, VarOptions{Flags: []string{"cluster_name=alpha"}})
	require.NoError(t, err)

	b, err := s.Run(bg, ref, vs)
	require.NoError(t, err)
	assert.Equal(t, "PROJ-PROV12-10", b.Key)

	require.Len(t, p.Triggered, 1)
	assert.Equal(t, "PROJ-PROV12", p.Triggered[0].PlanKey)
	assert.Equal(t, map[string]string{"cluster_name": "alpha", "compute_nodes": "2", "db_password": "hunter2"}, p.Triggered[0].Variables)
	assert.Equal(t, map[string]bool{"db_password": true}, p.Triggered[0].Secret)

	rec, ok, _ := s.State.Last(s.Cfg.RepoRoot)
	require.True(t, ok)
	assert.Equal(t, LastRecord{BuildKey: "PROJ-PROV12-10", Origin: s.Origin, PlanKey: "PROJ-PROV12", Target: "provision-lab",
		TriggeredAt: time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)}, rec)
}

func TestRunErrorRecordsNothing(t *testing.T) {
	p := fakeBamboo()
	p.TriggerErr = errs.Bamboof("could not start build of PROJ-BUILD")
	s := newService(t, p)
	ref, _ := s.ResolvePlan(bg, "PROJ-BUILD", "")
	_, err := s.Run(bg, ref, VarSet{})
	require.Error(t, err)
	_, ok, _ := s.State.Last(s.Cfg.RepoRoot)
	assert.False(t, ok)
}

func build(state provider.State, stageState, jobState provider.State) provider.Build {
	return provider.Build{Key: "PROJ-P-1", State: state, Stages: []provider.Stage{
		{Name: "Build", State: stageState, Jobs: []provider.Job{{Key: "PROJ-P-JOB1-1", Name: "Compile", State: jobState}}},
	}}
}

func collect(ch <-chan Event) []Event {
	var out []Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

func TestWatchEventsAndBackoff(t *testing.T) {
	p := fakeBamboo()
	q := provider.StateQueued
	p.Sequences = map[string][]provider.Build{"PROJ-P-1": {
		build(q, provider.StateNotBuilt, provider.StateNotBuilt),
		build(q, provider.StateNotBuilt, provider.StateNotBuilt),
		build(q, provider.StateNotBuilt, provider.StateNotBuilt),
		build(provider.StateRunning, provider.StateRunning, provider.StateRunning),
		build(provider.StateRunning, provider.StateRunning, provider.StateRunning),
		build(provider.StateSuccess, provider.StateSuccess, provider.StateSuccess),
	}}
	s := newService(t, p)
	events := collect(s.Watch(bg, "PROJ-P-1"))

	var kinds []string
	for _, e := range events {
		kinds = append(kinds, string(e.Type)+":"+string(e.State))
	}
	assert.Equal(t, []string{
		"state:queued",
		"state:running", "stage:running", "job:running",
		"state:success", "stage:success", "job:success",
		"done:success",
	}, kinds)
	assert.Equal(t, "Build", events[2].Name)
	assert.Equal(t, "PROJ-P-JOB1-1", events[3].Key)
	assert.Equal(t, provider.StateSuccess, events[len(events)-1].Build.State)

	clock := s.Clock.(*fakeClock)
	assert.Equal(t, []time.Duration{2 * time.Second, 3 * time.Second, 4500 * time.Millisecond, 2 * time.Second, 3 * time.Second}, clock.Waits())
}

func TestWatchBackoffCapsAtTenSeconds(t *testing.T) {
	p := fakeBamboo()
	var seq []provider.Build
	for i := 0; i < 8; i++ {
		seq = append(seq, provider.Build{Key: "PROJ-P-1", State: provider.StateQueued})
	}
	seq = append(seq, provider.Build{Key: "PROJ-P-1", State: provider.StateFailed})
	p.Sequences = map[string][]provider.Build{"PROJ-P-1": seq}
	s := newService(t, p)
	events := collect(s.Watch(bg, "PROJ-P-1"))
	assert.Equal(t, EventDone, events[len(events)-1].Type)
	waits := s.Clock.(*fakeClock).Waits()
	assert.Equal(t, 6750*time.Millisecond, waits[3])
	for _, w := range waits[4:] {
		assert.Equal(t, 10*time.Second, w)
	}
}

func TestWatchFinishedBuildEmitsStateThenDone(t *testing.T) {
	p := fakeBamboo()
	s := newService(t, p)
	events := collect(s.Watch(bg, "PROJ-BUILD-482"))
	require.Len(t, events, 2)
	assert.Equal(t, EventState, events[0].Type)
	assert.Equal(t, EventDone, events[1].Type)
	assert.Empty(t, s.Clock.(*fakeClock).Waits())
}

func TestWatchErrorEndsWithErrorEvent(t *testing.T) {
	s := newService(t, fakeBamboo())
	events := collect(s.Watch(bg, "PROJ-NOPE-1"))
	require.Len(t, events, 1)
	assert.Equal(t, EventError, events[0].Type)
	assert.True(t, errors.Is(events[0].Err, errs.ErrNotFound))
}

func TestWatchCancelStopsWatchingButNotTheBuild(t *testing.T) {
	p := fakeBamboo()
	p.Sequences = map[string][]provider.Build{"PROJ-P-1": {{Key: "PROJ-P-1", State: provider.StateRunning}}}
	s := newService(t, p)
	ctx, cancel := context.WithCancel(bg)
	ch := s.Watch(ctx, "PROJ-P-1")
	first := <-ch
	assert.Equal(t, EventState, first.Type)
	cancel()
	for e := range ch {
		assert.NotEqual(t, EventDone, e.Type)
	}
	assert.Empty(t, p.Stopped, "interrupting a watch never stops the build")
}
