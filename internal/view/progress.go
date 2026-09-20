package view

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/r0jjames/bam-cli/internal/provider"
)

// Bar renders a progress bar, a percentage and the time left. It returns ""
// when there is no estimate, so callers may append it unconditionally.
func Bar(o Out, p provider.Progress) string {
	if !p.Valid || p.Average <= 0 {
		return ""
	}
	cells := 20
	if o.Width > 0 {
		cells = min(24, max(10, o.Width/4))
	}
	filled := int(math.Round(p.Percent * float64(cells)))
	filled = min(max(filled, 0), cells)
	full, empty := "#", "-"
	if o.Style.Color {
		full, empty = "█", "░"
	}
	tail := "~" + Duration(p.Remaining) + " left"
	if p.Elapsed >= p.Average {
		tail = "over by " + Duration(p.Elapsed-p.Average)
	}
	return fmt.Sprintf("[%s%s] %3.0f%%  %s",
		strings.Repeat(full, filled), strings.Repeat(empty, cells-filled), p.Percent*100, tail)
}

// advance moves an estimate forward by d, so a live display keeps counting
// between polls. The server's own numbers replace it on the next poll.
func advance(p provider.Progress, d time.Duration) provider.Progress {
	if !p.Valid || p.Average <= 0 || d <= 0 {
		return p
	}
	p.Elapsed += d
	p.Remaining = max(0, p.Remaining-d)
	p.Percent = min(max(p.Percent+float64(d)/float64(p.Average), 0), 1)
	return p
}
