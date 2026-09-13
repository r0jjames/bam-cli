package view

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                "–",
		12 * time.Second:                 "12s",
		2*time.Minute + 3*time.Second:    "2m03s",
		time.Hour + 4*time.Minute + 59e9: "1h04m",
	}
	for d, want := range cases {
		assert.Equal(t, want, Duration(d), d.String())
	}
}

func TestAgo(t *testing.T) {
	assert.Equal(t, "40s ago", Ago(40*time.Second))
	assert.Equal(t, "18m ago", Ago(18*time.Minute))
	assert.Equal(t, "3h ago", Ago(3*time.Hour+10*time.Minute))
	assert.Equal(t, "2d ago", Ago(50*time.Hour))
}

func TestOutTimeByMode(t *testing.T) {
	tty, _ := testOut(true)
	pipe, _ := testOut(false)
	ts := fixedNow.Add(-18 * time.Minute)
	assert.Equal(t, "18m ago", tty.Time(ts))
	assert.Equal(t, "2026-09-12T11:42:00Z", pipe.Time(ts))
	assert.Equal(t, "–", tty.Time(time.Time{}))
}
