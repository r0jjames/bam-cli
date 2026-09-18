package tui

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
)

// buildsPerPlan is how many builds the Builds panel asks for.
const buildsPerPlan = 25

type connectedMsg struct {
	Alias string
	Svc   *app.Service
	Info  provider.ServerInfo
	User  provider.User
}

type plansLoadedMsg struct{ Plans []provider.Plan }

type buildsLoadedMsg struct {
	PlanKey string
	Builds  []provider.Build
}

type presetsLoadedMsg struct{ Targets []app.TargetInfo }

// errMsg is a failure to show, never a failure to exit on. Where names the
// panel so the status line can say what did not load.
type errMsg struct {
	Err   error
	Where string
}

func connectCmd(d Deps, alias string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		svc, err := d.Connect(ctx, alias)
		if err != nil {
			return errMsg{Err: err, Where: "connect"}
		}
		info, err := svc.P.ServerInfo(ctx)
		if err != nil {
			return errMsg{Err: err, Where: "connect"}
		}
		user, err := svc.P.CurrentUser(ctx)
		if err != nil {
			return errMsg{Err: err, Where: "connect"}
		}
		return connectedMsg{Alias: alias, Svc: svc, Info: info, User: user}
	}
}

// loadPlansCmd flattens every project's plans into one list, because the
// Plans panel is flat and filtered rather than nested (spec §3). An empty
// project means every project.
func loadPlansCmd(ctx context.Context, svc *app.Service, project string) tea.Cmd {
	return func() tea.Msg {
		projects, err := svc.P.ListProjects(ctx)
		if err != nil {
			return errMsg{Err: err, Where: "plans"}
		}
		var out []provider.Plan
		for _, p := range projects {
			if project != "" && p.Key != project {
				continue
			}
			plans, err := svc.P.ListPlans(ctx, p.Key)
			if err != nil {
				return errMsg{Err: err, Where: "plans"}
			}
			out = append(out, plans...)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
		return plansLoadedMsg{Plans: out}
	}
}

func loadBuildsCmd(ctx context.Context, svc *app.Service, planKey string, limit int) tea.Cmd {
	return func() tea.Msg {
		builds, err := svc.P.ListBuilds(ctx, planKey, provider.ListOptions{Limit: limit})
		if err != nil {
			return errMsg{Err: err, Where: "builds"}
		}
		return buildsLoadedMsg{PlanKey: planKey, Builds: builds}
	}
}

func loadPresetsCmd(d Deps) tea.Cmd {
	return func() tea.Msg {
		ts, err := d.Targets()
		if err != nil {
			return errMsg{Err: err, Where: "presets"}
		}
		return presetsLoadedMsg{Targets: ts}
	}
}

type logsLoadedMsg struct {
	JobKey string
	Title  string
	URL    string
	Lines  []string
	All    bool
}

// loadLogsCmd reads one build's logs. An empty jobKey with all=false means the
// failed jobs, which is what bam logs --last --failed prints.
func loadLogsCmd(ctx context.Context, svc *app.Service, b provider.Build, jobKey string, all bool) tea.Cmd {
	return func() tea.Msg {
		jobs, err := svc.Logs(ctx, b, app.LogsOptions{Failed: !all && jobKey == "", Job: jobKey})
		if err != nil {
			return errMsg{Err: err, Where: "logs"}
		}
		if len(jobs) == 0 {
			return errMsg{Err: errs.Bamboof("no failed job in %s", b.Key).
				WithTry("press a for every job's log"), Where: "logs"}
		}
		var lines []string
		for i, j := range jobs {
			if len(jobs) > 1 {
				if i > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, "== "+j.Job.Name+" ==")
			}
			lines = append(lines, j.Lines...)
		}
		title := jobs[0].Job.Key
		if len(jobs) > 1 {
			title = b.Key
		}
		return logsLoadedMsg{JobKey: jobs[0].Job.Key, Title: title, URL: jobs[0].Job.URL, Lines: lines, All: all}
	}
}

type buildLoadedMsg struct{ Build provider.Build }

// reloadBuildCmd re-reads one build. It goes through Service.Build rather
// than the provider directly, so a build the server has dropped is forgotten
// the same way the commands forget it.
func reloadBuildCmd(ctx context.Context, svc *app.Service, key string) tea.Cmd {
	return func() tea.Msg {
		b, err := svc.Build(ctx, key)
		if err != nil {
			return errMsg{Err: err, Where: "build"}
		}
		return buildLoadedMsg{Build: b}
	}
}

type projectsLoadedMsg struct{ Projects []provider.Project }

func loadProjectsCmd(ctx context.Context, svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		ps, err := svc.P.ListProjects(ctx)
		if err != nil {
			return errMsg{Err: err, Where: "projects"}
		}
		return projectsLoadedMsg{Projects: ps}
	}
}

type branchesLoadedMsg struct {
	MasterKey string
	Branches  []provider.Branch
}

func loadBranchesCmd(ctx context.Context, svc *app.Service, masterKey string) tea.Cmd {
	return func() tea.Msg {
		bs, err := svc.P.ListBranches(ctx, masterKey)
		if err != nil {
			return errMsg{Err: err, Where: "branches"}
		}
		return branchesLoadedMsg{MasterKey: masterKey, Branches: bs}
	}
}

type watchEventMsg struct{ Event app.Event }

// watchClosedMsg says the watch channel ended without a done event, which
// happens when the context was cancelled.
type watchClosedMsg struct{}

// watchCmd takes exactly one event off the channel. The model re-issues it
// after each watchEventMsg, which is bubbletea's channel pattern: Update
// stays pure and nothing ranges over a channel inside it.
func watchCmd(ch <-chan app.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return watchClosedMsg{}
		}
		return watchEventMsg{Event: e}
	}
}
