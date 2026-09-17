package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFinishedStates(t *testing.T) {
	finished := map[State]bool{
		StateSuccess: true, StateFailed: true, StateStopped: true, StateNotBuilt: true,
		StateQueued: false, StateRunning: false, StateSkipped: false, StateUnknown: false,
	}
	for s, want := range finished {
		assert.Equal(t, want, s.Finished(), string(s))
	}
	assert.Len(t, AllStates(), 8)
}

func TestFailedJobsWalksStagesInOrder(t *testing.T) {
	b := Build{Stages: []Stage{
		{Name: "Build", Jobs: []Job{{Key: "PROJ-P-JOB1-4", State: StateSuccess}}},
		{Name: "Test", Jobs: []Job{
			{Key: "PROJ-P-UNIT-4", State: StateFailed},
			{Key: "PROJ-P-INT-4", State: StateFailed},
		}},
	}}
	got := b.FailedJobs()
	assert.Equal(t, []string{"PROJ-P-UNIT-4", "PROJ-P-INT-4"}, []string{got[0].Key, got[1].Key})
}

func TestRevisionShort(t *testing.T) {
	assert.Equal(t, "a1b2c3d", Revision{Revision: "a1b2c3d4e5f6"}.Short())
	assert.Equal(t, "abc", Revision{Revision: "abc"}.Short())
}
