package bamboo

import (
	"testing"
	"time"

	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/assert"
)

func TestMapState(t *testing.T) {
	cases := []struct {
		life, state string
		want        provider.State
	}{
		{"Queued", "Unknown", provider.StateQueued},
		{"Pending", "Unknown", provider.StateQueued},
		{"InProgress", "Unknown", provider.StateRunning},
		{"Finished", "Successful", provider.StateSuccess},
		{"Finished", "Failed", provider.StateFailed},
		{"Finished", "Unknown", provider.StateStopped},
		{"NotBuilt", "Unknown", provider.StateNotBuilt},
		{"Skipped", "", provider.StateSkipped},
		{"Mystery", "Successful", provider.StateUnknown},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, mapState(tc.life, tc.state), tc.life+"/"+tc.state)
	}
}

func TestPlainReason(t *testing.T) {
	assert.Equal(t, "Manual run by J Doe", plainReason(`Manual run by <a href="https://bamboo.example.com/browse/user/jdoe">J Doe</a>`))
	assert.Equal(t, "Changes by a1b2c3d <jdoe@example.com>", plainReason("Changes by a1b2c3d &lt;jdoe@example.com&gt;"))
	assert.Equal(t, "Scheduled", plainReason("  Scheduled \n"))
}

func TestParseTime(t *testing.T) {
	want := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	assert.True(t, want.Equal(parseTime("2026-09-12T11:00:00.000+02:00")))
	assert.True(t, want.Equal(parseTime("2026-09-12T09:00:00.000Z")))
	assert.True(t, want.Equal(parseTime("2026-09-12T09:00:00Z")))
	assert.True(t, parseTime("").IsZero())
}
