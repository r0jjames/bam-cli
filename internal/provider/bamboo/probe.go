package bamboo

import (
	"context"
	"errors"
	"fmt"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
)

type ProbeStatus string

const (
	ProbeOK          ProbeStatus = "ok"
	ProbeUnsupported ProbeStatus = "unsupported"
	ProbeSkipped     ProbeStatus = "skipped"
	ProbeError       ProbeStatus = "error"
)

// ProbeResult is one line of bam doctor's capability report.
type ProbeResult struct {
	Name   string
	Status ProbeStatus
	Detail string
	Err    error
}

// Probe re-checks every optional capability against planKey and its latest
// builds, and rewrites the cache. Build stop is never probed: it has side
// effects, so it is learned the first time bam build cancel runs.
func (c *Client) Probe(ctx context.Context, planKey string) []ProbeResult {
	c.mu.Lock()
	stop := c.caps.Stop
	c.caps = Capabilities{Stop: stop}
	c.versionChecked = true
	c.mu.Unlock()

	var out []ProbeResult
	add := func(name string, err error, okDetail, unsupportedDetail string) {
		switch {
		case err == nil:
			out = append(out, ProbeResult{Name: name, Status: ProbeOK, Detail: okDetail})
		case errors.Is(err, errs.ErrUnsupported):
			out = append(out, ProbeResult{Name: name, Status: ProbeUnsupported, Detail: unsupportedDetail})
		default:
			out = append(out, ProbeResult{Name: name, Status: ProbeError, Detail: err.Error(), Err: err})
		}
	}
	skip := func(name, why string) { out = append(out, ProbeResult{Name: name, Status: ProbeSkipped, Detail: why}) }

	info, err := c.ServerInfo(ctx)
	add("server version", err, info.Version, "")

	vars, err := c.ListVariables(ctx, planKey)
	add("plan variables", err, fmt.Sprintf("%d declared on %s", len(vars), planKey),
		"names come from past builds and target defaults")

	builds, err := c.ListBuilds(ctx, planKey, provider.ListOptions{Limit: 1})
	if err != nil || len(builds) == 0 {
		why := planKey + " has no builds"
		if err != nil {
			why = err.Error()
		}
		skip("build variables", why)
		skip("logs", why)
	} else {
		latest := builds[0].Key
		_, err = c.BuildVariables(ctx, latest)
		add("build variables", err, "read from "+latest, "--from works only for builds bam triggered")

		b, err := c.GetBuild(ctx, latest)
		var job string
		if err == nil {
			for _, s := range b.Stages {
				if len(s.Jobs) > 0 {
					job = s.Jobs[0].Key
					break
				}
			}
		}
		if job == "" {
			skip("logs", latest+" has no jobs")
		} else {
			_, err = c.FetchLog(ctx, job, provider.LogOptions{})
			add("logs", err, "via "+c.Capabilities().Log, "")
		}
	}

	failed, err := c.ListBuilds(ctx, planKey, provider.ListOptions{Limit: 1, State: provider.StateFailed})
	if err != nil || len(failed) == 0 {
		skip("failed tests", "no failed build of "+planKey+" to read")
	} else {
		c.failedTests(ctx, failed[0].Key)
		switch c.Capabilities().FailedTests {
		case "yes":
			add("failed tests", nil, "read from "+failed[0].Key, "")
		case "no":
			add("failed tests", errs.ErrUnsupported, "", "build show lists failed jobs only")
		default:
			skip("failed tests", "could not read "+failed[0].Key)
		}
	}

	switch stop {
	case "yes":
		add("stop builds", nil, "confirmed by an earlier cancel", "")
	case "no":
		add("stop builds", errs.ErrUnsupported, "", "bam build cancel is unavailable")
	default:
		skip("stop builds", "learned the first time bam build cancel runs")
	}
	return out
}
