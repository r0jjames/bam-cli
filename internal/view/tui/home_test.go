package tui

import (
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/require"
)

var homeNow = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func withBuild(p provider.Plan, n int, s provider.State, ago time.Duration, reason string) provider.Plan {
	p.LastBuild = &provider.BuildSummary{Key: p.Key + "-" + strconv.Itoa(n), Number: n, State: s,
		FinishedAt: homeNow.Add(-ago), Reason: reason}
	return p
}

func sortedKeys(ps []provider.Plan, less func(a, b provider.Plan) bool) []string {
	out := append([]provider.Plan(nil), ps...)
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	keys := []string{}
	for _, p := range out {
		keys = append(keys, p.Key)
	}
	return keys
}

func samplePlans() []provider.Plan {
	return []provider.Plan{
		withBuild(provider.Plan{Key: "PROJ-BUILD", Name: "Build and test", ProjectKey: "PROJ"}, 4, provider.StateFailed, 12*time.Minute, "Manual run by J Doe"),
		withBuild(provider.Plan{Key: "PROJ-PROV", Name: "Provision lab", ProjectKey: "PROJ"}, 8, provider.StateSuccess, 3*time.Hour, "Scheduled"),
		withBuild(provider.Plan{Key: "OPS-NIGHTLY", Name: "Nightly", ProjectKey: "OPS"}, 9, provider.StateSuccess, 30*time.Second, "Changes by a1b2c3d <jdoe@example.com>"),
		{Key: "OPS-OLD", Name: "Archived", ProjectKey: "OPS"},
	}
}

func TestPlanLessColumns(t *testing.T) {
	ps := samplePlans()
	require.Equal(t, []string{"OPS-NIGHTLY", "OPS-OLD", "PROJ-BUILD", "PROJ-PROV"}, sortedKeys(ps, planLess(homeSort{})),
		"default: project, then key")
	require.Equal(t, []string{"OPS-OLD", "PROJ-BUILD", "OPS-NIGHTLY", "PROJ-PROV"}, sortedKeys(ps, planLess(homeSort{col: colName})))
	require.Equal(t, []string{"PROJ-BUILD", "OPS-NIGHTLY", "PROJ-PROV", "OPS-OLD"}, sortedKeys(ps, planLess(homeSort{col: colState})),
		"failed first; equal states break on project then key; no build last")
	require.Equal(t, []string{"OPS-NIGHTLY", "PROJ-BUILD", "PROJ-PROV", "OPS-OLD"}, sortedKeys(ps, planLess(homeSort{col: colAge})),
		"newest first; no build last")
	require.Equal(t, []string{"PROJ-PROV", "PROJ-BUILD", "OPS-OLD", "OPS-NIGHTLY"}, sortedKeys(ps, planLess(homeSort{col: colKey, desc: true})))
}

func TestHomeSortCyclesAndLabels(t *testing.T) {
	s := homeSort{}
	require.Equal(t, "key↑", s.label(homePlans))
	s = s.next()
	require.Equal(t, homeSort{col: colName}, s)
	require.Equal(t, "plan↑", s.label(homePresets))
	require.Equal(t, homeSort{}, homeSort{col: colAge, desc: true}.next(), "after age comes key again, ascending")
	require.Equal(t, "age↓", homeSort{col: colAge, desc: true}.label(homePlans))
}

func TestParseSort(t *testing.T) {
	s, err := parseSort(homePlans, "-age")
	require.NoError(t, err)
	require.Equal(t, homeSort{col: colAge, desc: true}, s)
	s, err = parseSort(homePresets, "target")
	require.NoError(t, err)
	require.Equal(t, homeSort{col: colKey}, s)
	_, err = parseSort(homePlans, "target")
	require.EqualError(t, err, `unknown sort column "target"; columns: key name state age`)
}

func TestShortBy(t *testing.T) {
	for reason, want := range map[string]string{
		"Manual run by J Doe":                   "J Doe",
		"Scheduled":                             "sched",
		"Changes by a1b2c3d <jdoe@example.com>": "commit",
		"Code has changed":                      "commit",
		"Child of PROJ-BUILD-4":                 "dep",
		"Dependant of PROJ-BUILD":               "dep",
		"Rebuilt by jdoe":                       "Rebuilt",
		"":                                      "–",
	} {
		require.Equal(t, want, shortBy(reason), reason)
	}
}

func TestAgeShort(t *testing.T) {
	require.Equal(t, "now", ageShort(30*time.Second))
	require.Equal(t, "12m", ageShort(12*time.Minute))
	require.Equal(t, "3h", ageShort(3*time.Hour))
	require.Equal(t, "2d", ageShort(49*time.Hour))
}

func titles(cols []tableCol) string {
	out := []string{}
	for _, c := range cols {
		out = append(out, c.title)
	}
	return strings.Join(out, " ")
}

func TestPlanColumnsDropBYThenPROJECT(t *testing.T) {
	ps := samplePlans()
	require.Equal(t, "PROJECT KEY NAME STATE # AGE BY", titles(planColumns(120, ps)))
	require.Equal(t, "PROJECT KEY NAME STATE # AGE", titles(planColumns(90, ps)))
	require.Equal(t, "KEY NAME STATE # AGE", titles(planColumns(70, ps)))
	for _, w := range []int{120, 90, 70} {
		total := 2
		for i, c := range planColumns(w, ps) {
			if i > 0 {
				total += 2
			}
			total += c.width
		}
		require.LessOrEqual(t, total, w, "width %d", w)
	}
}

func TestBuildCells(t *testing.T) {
	st, num, age, by := buildCells(nil, nil, homeNow)
	require.Equal(t, []string{"–", "–", "–", "–"}, []string{st, num, age, by})

	last := &provider.BuildSummary{Number: 4, State: provider.StateFailed, FinishedAt: homeNow.Add(-12 * time.Minute), Reason: "Scheduled"}
	st, num, age, by = buildCells(last, nil, homeNow)
	require.Contains(t, st, "failed")
	require.Equal(t, []string{"4", "12m", "sched"}, []string{num, age, by})

	live := &provider.Build{Number: 5, State: provider.StateRunning, Reason: "Manual run by jdoe"}
	st, num, age, by = buildCells(last, live, homeNow)
	require.Contains(t, st, "running")
	require.Equal(t, []string{"5", "now", "jdoe"}, []string{num, age, by}, "a live build wins over the last finished one")

	done := &provider.Build{Number: 5, State: provider.StateSuccess}
	st, _, _, _ = buildCells(last, done, homeNow)
	require.Contains(t, st, "failed", "a finished live build is not live")
}

func TestTableLinesPadsAlignsAndMarksTheCursor(t *testing.T) {
	cols := []tableCol{{title: "KEY", width: 6}, {title: "#", width: 3, right: true}, {title: "NAME", width: 5}}
	lines := tableLines(cols, [][]string{{"A", "4", "Longer name"}, {"B", "12", "x"}}, 1)
	require.Equal(t, []string{
		"  KEY       #  NAME",
		"  A         4  Long…",
		"▸ B        12  x",
	}, lines)
}

func TestPlanMatchTextIncludesProjectAndState(t *testing.T) {
	txt := planMatchText(samplePlans()[0])
	for _, want := range []string{"PROJ", "PROJ-BUILD", "Build and test", "failed"} {
		require.Contains(t, txt, want)
	}
}

func TestPlanCellsFollowsTheColumns(t *testing.T) {
	p := samplePlans()[0] // PROJ-BUILD, failed #4, 12m, Manual run by J Doe
	cols := planColumns(120, samplePlans())
	cells := planCells(cols, p, nil, homeNow)
	require.Len(t, cells, len(cols))
	got := map[string]string{}
	for i, c := range cols {
		got[c.title] = cells[i]
	}
	require.Equal(t, "PROJ", got["PROJECT"])
	require.Equal(t, "PROJ-BUILD", got["KEY"])
	require.Equal(t, "Build and test", got["NAME"])
	require.Contains(t, got["STATE"], "failed")
	require.Equal(t, "4", got["#"])
	require.Equal(t, "12m", got["AGE"])
	require.Equal(t, "J Doe", got["BY"])

	narrow := planColumns(70, samplePlans())
	require.Len(t, planCells(narrow, p, nil, homeNow), len(narrow), "cells follow the dropped columns")
}
