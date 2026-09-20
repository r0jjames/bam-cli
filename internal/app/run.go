package app

import (
	"context"
	"errors"
	"time"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
)

// Polling bounds (spec §5.4).
const (
	MinPoll = 2 * time.Second
	MaxPoll = 10 * time.Second
)

// Run triggers the plan with the variables that differ from the plan's
// values and records the build as this repository's last build.
func (s *Service) Run(ctx context.Context, ref PlanRef, vs VarSet) (provider.Build, error) {
	b, err := s.P.Trigger(ctx, provider.TriggerRequest{PlanKey: ref.PlanKey, Variables: vs.Changed(), Secret: vs.Secret()})
	if err != nil {
		return provider.Build{}, err
	}
	rec := LastRecord{BuildKey: b.Key, Origin: s.Origin, PlanKey: ref.PlanKey, TriggeredAt: s.Clock.Now()}
	if ref.Target != nil {
		rec.Target = ref.Target.Name
	}
	// The build is already running; failing to remember it only loses --last.
	_ = s.State.SetLast(s.Cfg.RepoRoot, rec)
	return b, nil
}

// EventType names what changed between two polls.
type EventType string

const (
	EventState EventType = "state" // the build's state changed (always sent for the first poll)
	EventStage EventType = "stage" // a stage's state changed
	EventJob   EventType = "job"   // a job's state changed
	EventDone  EventType = "done"  // the build finished; last event
	EventError EventType = "error" // polling failed; last event
)

// Event is one change seen while watching. Build is the snapshot of that poll,
// and Progress the server's estimate for it; a zero Progress means none.
type Event struct {
	Type     EventType
	Time     time.Time
	Build    provider.Build
	Progress provider.Progress
	Name     string // stage or job name
	Key      string // job result key for job events
	State    provider.State
	Err      error
}

// Watch polls a build until it finishes and emits its changes. Ending ctx
// stops watching only; the build keeps running.
func (s *Service) Watch(ctx context.Context, key string) <-chan Event {
	ch := make(chan Event, 16)
	go func() {
		defer close(ch)
		send := func(e Event) bool {
			select {
			case ch <- e:
				return true
			case <-ctx.Done():
				return false
			}
		}
		var prev *provider.Build
		interval := MinPoll
		noProgress := false
		for {
			// The provider ignores ctx, so check here before every poll:
			// ending ctx must stop watching, not keep polling forever.
			select {
			case <-ctx.Done():
				return
			default:
			}
			b, err := s.Build(ctx, key)
			if err != nil {
				if ctx.Err() == nil {
					send(Event{Type: EventError, Time: s.Clock.Now(), Err: err})
				}
				return
			}
			now := s.Clock.Now()
			// A missing estimate is decoration lost, never a reason to stop
			// watching, so every progress error becomes a zero Progress.
			var pr provider.Progress
			if !noProgress && !b.State.Finished() {
				got, err := s.P.BuildProgress(ctx, key)
				switch {
				case err == nil:
					pr = got
				case errors.Is(err, errs.ErrUnsupported):
					noProgress = true
				}
			}
			changes := diff(prev, b)
			for _, e := range changes {
				e.Time, e.Build, e.Progress = now, b, pr
				if !send(e) {
					return
				}
			}
			if b.State.Finished() {
				send(Event{Type: EventDone, Time: now, Build: b, State: b.State})
				return
			}
			if len(changes) > 0 {
				interval = MinPoll
			} else {
				interval = min(interval*3/2, MaxPoll)
			}
			prev = &b
			select {
			case <-ctx.Done():
				return
			case <-s.Clock.After(interval):
			}
		}
	}()
	return ch
}

// diff lists the changes from prev to cur. The first poll reports only the
// build state.
func diff(prev *provider.Build, cur provider.Build) []Event {
	if prev == nil {
		return []Event{{Type: EventState, State: cur.State}}
	}
	var out []Event
	if prev.State != cur.State {
		out = append(out, Event{Type: EventState, State: cur.State})
	}
	oldStages := map[string]provider.State{}
	oldJobs := map[string]provider.State{}
	for _, st := range prev.Stages {
		oldStages[st.Name] = st.State
		for _, j := range st.Jobs {
			oldJobs[j.Key] = j.State
		}
	}
	for _, st := range cur.Stages {
		if was, ok := oldStages[st.Name]; (ok && was != st.State) || (!ok && st.State != provider.StateNotBuilt) {
			out = append(out, Event{Type: EventStage, Name: st.Name, State: st.State})
		}
		for _, j := range st.Jobs {
			if was, ok := oldJobs[j.Key]; (ok && was != j.State) || (!ok && j.State != provider.StateNotBuilt) {
				out = append(out, Event{Type: EventJob, Name: j.Name, Key: j.Key, State: j.State})
			}
		}
	}
	return out
}
