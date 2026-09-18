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
	Gen   int
	Alias string
	Svc   *app.Service
	Info  provider.ServerInfo
	User  provider.User
}

// Every asynchronous load carries the generation of the request that started
// it. The model bumps a stream's generation whenever it issues a new request
// or abandons one, and drops any message from an older generation: otherwise
// a slow response for the server, project, branch or build the user has just
// left lands on the one they are now looking at.
type plansLoadedMsg struct {
	Gen   int
	Plans []provider.Plan
}

type buildsLoadedMsg struct {
	Gen     int
	PlanKey string
	Builds  []provider.Build
}

type presetsLoadedMsg struct {
	Gen     int
	Targets []app.TargetInfo
}

// errMsg is a failure to show, never a failure to exit on. Where names the
// panel so the status line can say what did not load.
type errMsg struct {
	Err   error
	Where string
}

func connectCmd(ctx context.Context, d Deps, alias string, gen int) tea.Cmd {
	return func() tea.Msg {
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
		return connectedMsg{Gen: gen, Alias: alias, Svc: svc, Info: info, User: user}
	}
}

// loadPlansCmd flattens every project's plans into one list, because the
// Plans panel is flat and filtered rather than nested (spec §3). An empty
// project means every project.
func loadPlansCmd(ctx context.Context, svc *app.Service, project string, gen int) tea.Cmd {
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
		return plansLoadedMsg{Gen: gen, Plans: out}
	}
}

func loadBuildsCmd(ctx context.Context, svc *app.Service, planKey string, limit int, gen int) tea.Cmd {
	return func() tea.Msg {
		builds, err := svc.P.ListBuilds(ctx, planKey, provider.ListOptions{Limit: limit})
		if err != nil {
			return errMsg{Err: err, Where: "builds"}
		}
		return buildsLoadedMsg{Gen: gen, PlanKey: planKey, Builds: builds}
	}
}

func loadPresetsCmd(d Deps, gen int) tea.Cmd {
	return func() tea.Msg {
		ts, err := d.Targets()
		if err != nil {
			return errMsg{Err: err, Where: "presets"}
		}
		return presetsLoadedMsg{Gen: gen, Targets: ts}
	}
}

type logsLoadedMsg struct {
	Gen    int
	JobKey string
	Title  string
	URL    string
	Lines  []string
	All    bool
	Multi  bool // several jobs are concatenated in Lines
	Offset int  // the provider's next-read offset, for follow
}

// loadLogsCmd reads one build's logs. An empty jobKey with all=false means the
// failed jobs, which is what bam logs --last --failed prints.
func loadLogsCmd(ctx context.Context, svc *app.Service, b provider.Build, jobKey string, all bool, gen int) tea.Cmd {
	return func() tea.Msg {
		jobs, err := svc.Logs(ctx, b, app.LogsOptions{Failed: !all && jobKey == "", Job: jobKey})
		if err != nil {
			return errMsg{Err: err, Where: "logs"}
		}
		if len(jobs) == 0 {
			// Say what is actually missing: advising "press a" when a was
			// already pressed is worse than saying nothing.
			if all {
				return errMsg{Err: errs.Bamboof("%s has no job logs", b.Key).
					WithWhy("the build has no jobs, or the server kept no log for them"), Where: "logs"}
			}
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
		// Next is the provider's offset contract for the next read. It is
		// meaningful only for a single job: a concatenation of several has no
		// one offset to resume from.
		return logsLoadedMsg{Gen: gen, JobKey: jobs[0].Job.Key, Title: title, URL: jobs[0].Job.URL,
			Lines: lines, All: all, Multi: len(jobs) > 1, Offset: jobs[0].Next}
	}
}

type logChunkMsg struct {
	Gen   int
	Lines []string
}

type followEndedMsg struct {
	Gen int
	Err error
}

// followCmd runs app.FollowLog in its own goroutine and adapts its emit
// callback into a channel. FollowLog blocks until the job finishes, so this
// is the one place the UI starts a goroutine, and ending ctx ends it.
func followCmd(ctx context.Context, svc *app.Service, buildKey string, job provider.Job, offset int) (<-chan []string, <-chan error) {
	lines := make(chan []string, 8)
	done := make(chan error, 1)
	go func() {
		err := svc.FollowLog(ctx, buildKey, job, offset, func(ls []string) {
			select {
			case lines <- ls:
			case <-ctx.Done():
			}
		})
		// The error is published before the channel closes, so a drain that
		// sees the close always finds the outcome waiting. Closing first
		// would let a real failure be read as a clean end.
		done <- err
		close(lines)
	}()
	return lines, done
}

// drainFollowCmd takes one chunk off the channel, or reports the end. The
// model re-issues it after each chunk, so Update never ranges over a channel.
func drainFollowCmd(lines <-chan []string, done <-chan error, gen int) tea.Cmd {
	return func() tea.Msg {
		if ls, ok := <-lines; ok {
			return logChunkMsg{Gen: gen, Lines: ls}
		}
		select {
		case err := <-done:
			return followEndedMsg{Gen: gen, Err: err}
		default:
			return followEndedMsg{Gen: gen}
		}
	}
}

// resolveTargetBuildsCmd resolves a preset to its plan, honouring the branch
// the preset names, and then lists that plan's builds. It is one command so
// the two requests share a generation.
func resolveTargetBuildsCmd(ctx context.Context, svc *app.Service, target app.TargetInfo, gen int) tea.Cmd {
	return func() tea.Msg {
		// From the TargetInfo the panel is showing, not from the service's
		// configuration: the panel may have been refreshed since the UI
		// started, and the service's copy would be the older one.
		ref, err := svc.RefFromTarget(ctx, target)
		if err != nil {
			return errMsg{Err: err, Where: "presets"}
		}
		builds, err := svc.P.ListBuilds(ctx, ref.PlanKey, provider.ListOptions{Limit: buildsPerPlan})
		if err != nil {
			return errMsg{Err: err, Where: "builds"}
		}
		return buildsLoadedMsg{Gen: gen, PlanKey: ref.PlanKey, Builds: builds}
	}
}

// logsForKeyCmd reads a build in full and then its logs, for a row that came
// from a listing and therefore carries no stages.
func logsForKeyCmd(ctx context.Context, svc *app.Service, buildKey, jobKey string, all bool, gen int) tea.Cmd {
	return func() tea.Msg {
		b, err := svc.Build(ctx, buildKey)
		if err != nil {
			return errMsg{Err: err, Where: "logs"}
		}
		return loadLogsCmd(ctx, svc, b, jobKey, all, gen)()
	}
}

type buildLoadedMsg struct {
	Gen   int
	Build provider.Build
}

// reloadBuildCmd re-reads one build. It goes through Service.Build rather
// than the provider directly, so a build the server has dropped is forgotten
// the same way the commands forget it.
func reloadBuildCmd(ctx context.Context, svc *app.Service, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		b, err := svc.Build(ctx, key)
		if err != nil {
			return errMsg{Err: err, Where: "build"}
		}
		return buildLoadedMsg{Gen: gen, Build: b}
	}
}

type projectsLoadedMsg struct {
	Gen      int
	Projects []provider.Project
}

func loadProjectsCmd(ctx context.Context, svc *app.Service, gen int) tea.Cmd {
	return func() tea.Msg {
		ps, err := svc.P.ListProjects(ctx)
		if err != nil {
			return errMsg{Err: err, Where: "projects"}
		}
		return projectsLoadedMsg{Gen: gen, Projects: ps}
	}
}

type branchesLoadedMsg struct {
	Gen       int
	MasterKey string
	Branches  []provider.Branch
}

func loadBranchesCmd(ctx context.Context, svc *app.Service, masterKey string, gen int) tea.Cmd {
	return func() tea.Msg {
		bs, err := svc.P.ListBranches(ctx, masterKey)
		if err != nil {
			return errMsg{Err: err, Where: "branches"}
		}
		return branchesLoadedMsg{Gen: gen, MasterKey: masterKey, Branches: bs}
	}
}

type watchEventMsg struct {
	Gen   int
	Event app.Event
}

// watchClosedMsg says the watch channel ended without a done event, which
// happens when the context was cancelled.
type watchClosedMsg struct{ Gen int }

// watchCmd takes exactly one event off the channel. The model re-issues it
// after each watchEventMsg, which is bubbletea's channel pattern: Update
// stays pure and nothing ranges over a channel inside it.
func watchCmd(ch <-chan app.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return watchClosedMsg{Gen: gen}
		}
		return watchEventMsg{Gen: gen, Event: e}
	}
}
