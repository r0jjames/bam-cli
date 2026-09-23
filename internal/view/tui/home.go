package tui

import "time"

// homeState is what the Home screen keeps between frames (home spec §3-§4).
type homeState struct {
	loadedAt time.Time // the last load in which every project succeeded
	stale    bool      // the last load, or part of it, failed
}
