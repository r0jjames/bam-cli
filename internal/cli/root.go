package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/view"
	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X github.com/r0jjames/bam-cli/internal/cli.version=v0.1.0".
var version = "dev"

const (
	groupSetup    = "setup"
	groupBrowse   = "browse"
	groupRun      = "run"
	groupShortcut = "shortcuts"
)

// Execute runs bam with args and returns the process exit code.
func Execute(ctx context.Context, args []string, env Env) int {
	r := &runtime{env: env}
	root := newRoot(r)
	root.SetArgs(args)
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return errs.Usagef("%s", err.Error()).WithTry(c.CommandPath() + " --help")
	})
	err := root.ExecuteContext(ctx)
	var res *resultError
	var silent silentError
	if err != nil && !errors.As(err, &res) && !errors.As(err, &silent) && !errors.Is(err, context.Canceled) {
		view.PrintError(env.Stderr, err, r.flags.debug)
	}
	return exitCode(err)
}

func newRoot(r *runtime) *cobra.Command {
	root := &cobra.Command{
		Use:           "bam",
		Short:         "A terminal remote control for Atlassian Bamboo",
		Long:          "bam triggers, watches and diagnoses Bamboo builds from the terminal.\nRun bam alone on a terminal for the interactive UI (coming in v0.2).",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			if r.env.StdoutTTY {
				fmt.Fprintln(cmd.OutOrStdout(), "\ninteractive mode arrives in v0.2")
			}
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.StringVar(&r.flags.server, "server", "", "server alias to use")
	pf.BoolVar(&r.flags.json, "json", false, "print JSON")
	pf.StringVar(&r.flags.color, "color", "", "color output: auto, always or never")
	pf.BoolVar(&r.flags.debug, "debug", false, "log HTTP requests to stderr")

	root.AddGroup(
		&cobra.Group{ID: groupSetup, Title: "Setup:"},
		&cobra.Group{ID: groupBrowse, Title: "Browse:"},
		&cobra.Group{ID: groupRun, Title: "Run:"},
		&cobra.Group{ID: groupShortcut, Title: "Shortcuts:"},
	)
	root.AddCommand(newVersionCmd(r))
	for _, add := range commandSets {
		add(root, r)
	}
	return root
}

// commandSets register command groups; later tasks append to it from init().
var commandSets []func(root *cobra.Command, r *runtime)

func newVersionCmd(r *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the bam version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(r.env.Stdout, "bam %s\n", version)
			return err
		},
	}
}
