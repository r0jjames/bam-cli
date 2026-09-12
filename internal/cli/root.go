package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X github.com/r0jjames/bam-cli/internal/cli.version=v0.1.0".
var version = "dev"

// Execute runs bam with args and returns the process exit code.
func Execute(ctx context.Context, args []string, env Env) int {
	root := newRoot(env)
	root.SetArgs(args)
	root.SetIn(env.Stdin)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(env.Stderr, "error:", err)
		return 2
	}
	return 0
}

func newRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "bam",
		Short:         "A terminal remote control for Atlassian Bamboo",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(env))
	return root
}

func newVersionCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the bam version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(env.Stdout, "bam %s\n", version)
			return err
		},
	}
}
