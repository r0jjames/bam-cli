package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/credential"
	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/view"
	"github.com/spf13/cobra"
)

func init() { commandSets = append(commandSets, addPresets) }

func addPresets(root *cobra.Command, r *runtime) {
	subgroup(root, "target", "Run presets: list, show, add", groupRun).
		AddCommand(newTargetListCmd(r), newTargetShowCmd(r), newTargetAddCmd(r))
	c := newInitCmd(r)
	c.GroupID = groupSetup
	root.AddCommand(c)
}

// defaultTargetName is the lowercased plan part of a plan key.
func defaultTargetName(planKey string) string {
	if i := strings.Index(planKey, "-"); i >= 0 {
		return strings.ToLower(planKey[i+1:])
	}
	return strings.ToLower(planKey)
}

func newTargetListCmd(r *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List run presets from .bam.yaml and the machine config",
		Args:  cobra.NoArgs,
		RunE: r.wrap(func(cmd *cobra.Command, _ []string) error {
			cfg, err := r.config()
			if err != nil {
				return err
			}
			infos, err := app.DescribeTargets(cfg, r.env.Getenv)
			if err != nil {
				return err
			}
			if r.flags.json {
				docs := []view.TargetDoc{}
				for _, t := range infos {
					docs = append(docs, view.TargetJSON(t))
				}
				return view.WriteJSON(r.env.Stdout, docs)
			}
			if len(infos) == 0 {
				r.note("no targets configured", "bam target add <name> --plan PROJ-PLAN")
				return nil
			}
			o, err := r.out()
			if err != nil {
				return err
			}
			return view.Targets(o, infos)
		}),
	}
}

func newTargetShowCmd(r *runtime) *cobra.Command {
	return &cobra.Command{
		Use:               "show <name>",
		Short:             "Show a run preset with its defaults, options and origin",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: r.completeTargets,
		RunE: r.wrap(func(cmd *cobra.Command, args []string) error {
			cfg, err := r.config()
			if err != nil {
				return err
			}
			info, err := app.DescribeTarget(cfg, r.env.Getenv, args[0])
			if err != nil {
				return err
			}
			if r.flags.json {
				return view.WriteJSON(r.env.Stdout, view.TargetJSON(info))
			}
			o, err := r.out()
			if err != nil {
				return err
			}
			return view.TargetDetail(o, info)
		}),
	}
}

func newTargetAddCmd(r *runtime) *cobra.Command {
	var plan, from, branch string
	var machine, printOnly, force bool
	cmd := &cobra.Command{
		Use:   "add <name> --plan KEY",
		Short: "Generate a run preset from a plan's declared variables",
		Args:  cobra.ExactArgs(1),
		RunE: r.wrap(func(cmd *cobra.Command, args []string) error {
			if plan == "" {
				return errs.Usagef("--plan is required").WithTry(fmt.Sprintf("bam target add %s --plan PROJ-PLAN", args[0]))
			}
			svc, _, err := r.connect(cmd.Context(), "")
			if err != nil {
				return err
			}
			d, err := svc.GenerateTarget(cmd.Context(), app.GenerateOptions{Name: args[0], PlanKey: plan, From: from, Branch: branch})
			if err != nil {
				return err
			}
			if printOnly {
				_, err := fmt.Fprint(r.env.Stdout, config.DraftYAML(d))
				return err
			}
			path, keyPath := r.cfg.ProjectPath, []string{"targets"}
			if machine {
				path, keyPath = r.env.Paths.MachineConfig, []string{"repos", r.cfg.RepoRoot, "targets"}
			} else if path == "" {
				return errs.Usagef("no .bam.yaml in this repository").
					WithTry("bam init --server ALIAS --project KEY, or pass --machine")
			}
			if err := config.WriteTarget(path, keyPath, d, force); err != nil {
				return err
			}
			fmt.Fprintf(r.env.Stdout, "added target %s to %s\n", d.Name, path)
			if d.Note != "" {
				fmt.Fprintln(r.env.Stderr, "note: "+d.Note)
			}
			fmt.Fprintln(r.env.Stderr, "review before commit: values copied from Bamboo")
			return nil
		}),
	}
	f := cmd.Flags()
	f.StringVar(&plan, "plan", "", "plan key to generate from")
	f.StringVar(&from, "from", "", "build whose values fill the comments: number, key or last")
	f.StringVar(&branch, "branch", "", "default branch for the preset")
	f.BoolVar(&machine, "machine", false, "write to the machine config instead of .bam.yaml")
	f.BoolVar(&printOnly, "print", false, "print the YAML and write nothing")
	f.BoolVar(&force, "force", false, "replace an existing target")
	return cmd
}

func newInitCmd(r *runtime) *cobra.Command {
	var url string
	var projects, plans []string
	var force bool
	cmd := &cobra.Command{
		Use:   "init --server ALIAS --project KEY [--plan KEY[=name]]...",
		Short: "Write .bam.yaml for this repository, with generated presets",
		Args:  cobra.NoArgs,
		RunE: r.wrap(func(cmd *cobra.Command, _ []string) error {
			alias := r.flags.server
			if alias == "" {
				return errs.Usagef("--server ALIAS is required").WithTry("bam init --server work --project PROJ")
			}
			if len(projects) == 0 {
				return errs.Usagef("--project is required").WithTry("bam init --server " + alias + " --project PROJ")
			}
			cfg, err := r.config()
			if err != nil {
				return err
			}
			path := filepath.Join(cfg.RepoRoot, config.ProjectFileName)
			known := cfg.Servers()[alias]
			if url == "" {
				url = known.URL
			}
			if url == "" {
				return errs.Configf("no URL for server alias %q", alias).
					WithTry(fmt.Sprintf("pass --url, or bam server add %s --url URL", alias))
			}
			if _, err := credential.Origin(url); err != nil {
				return err
			}
			var drafts []config.TargetDraft
			if len(plans) > 0 {
				svc, _, err := r.connectServer(config.ResolvedServer{Alias: alias, URL: url, AuthEnv: known.AuthEnv})
				if err != nil {
					return err
				}
				for _, p := range plans {
					key, name, _ := strings.Cut(p, "=")
					if name == "" {
						name = defaultTargetName(key)
					}
					d, err := svc.GenerateTarget(cmd.Context(), app.GenerateOptions{Name: name, PlanKey: key})
					if err != nil {
						if errs.KindOf(err) == errs.KindUsage && !config.ValidTargetName(name) {
							return errs.Usagef("cannot derive a target name from %s", key).WithTry(fmt.Sprintf("--plan %s=NAME", key))
						}
						return err
					}
					drafts = append(drafts, d)
				}
			}
			if err := config.WriteInitFile(path, config.InitFile{ServerAlias: alias, ServerURL: url, Projects: projects}, drafts, force); err != nil {
				return err
			}
			fmt.Fprintf(r.env.Stdout, "wrote %s\n", path)
			if len(drafts) > 0 {
				fmt.Fprintln(r.env.Stderr, "review before commit: values copied from Bamboo")
			}
			return nil
		}),
	}
	f := cmd.Flags()
	f.StringVar(&url, "url", "", "server URL (default: the URL of --server in the machine config)")
	f.StringSliceVar(&projects, "project", nil, "project key (repeatable; the first is the default)")
	f.StringArrayVar(&plans, "plan", nil, "plan to generate a target from, KEY or KEY=name (repeatable)")
	f.BoolVar(&force, "force", false, "overwrite an existing .bam.yaml")
	return cmd
}
