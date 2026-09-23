package tui

import (
	"sort"
	"strings"

	"github.com/r0jjames/bam-cli/internal/provider"
)

// plansLoaded folds a plans load into the shared list. The rows of a project
// that failed this time are kept from the last load, the cursor stays on the
// plan it was on, and a configured project the server lacks is named.
func (m Model) plansLoaded(msg plansLoadedMsg) Model {
	m.plans.loading = false
	items := append([]provider.Plan(nil), msg.Plans...)
	if len(msg.Failed) > 0 {
		failed := map[string]bool{}
		for _, k := range msg.Failed {
			failed[k] = true
		}
		for _, p := range m.plans.items {
			if failed[p.ProjectKey] {
				items = append(items, p)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	}
	keep, had := m.plans.selected()
	m.plans.setItems(items)
	if had {
		m.plans.selectFirst(func(p provider.Plan) bool { return p.Key == keep.Key })
	}
	if msg.Err != nil {
		m.err = msg.Err
		m.home.stale = true
	} else {
		m.home.stale = false
		m.home.loadedAt = m.now()
	}
	if len(msg.Missing) > 0 {
		verb := "is"
		if len(msg.Missing) > 1 {
			verb = "are"
		}
		m.status = "project " + strings.Join(msg.Missing, ", ") + " " + verb + " configured but not on " + m.server
	}
	return m
}
