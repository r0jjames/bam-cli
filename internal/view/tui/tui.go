// Package tui is bam's terminal UI: a second view over the same app use
// cases the commands call. It never builds a provider and never reads
// configuration; cli injects everything through Deps.
package tui

import (
	"context"
	"io"

	"github.com/r0jjames/bam-cli/internal/app"
)

// Server is one entry of the server registry, as the UI needs it.
type Server struct {
	Alias string
	URL   string
}

// Deps is everything the UI needs. Every field is injectable, so the model
// tests need no network, no keychain and no terminal.
type Deps struct {
	Servers   []Server
	Initial   string // alias to open with
	Connect   func(ctx context.Context, alias string) (*app.Service, error)
	Targets   func() ([]app.TargetInfo, error)
	Open      func(url string) error
	Clipboard io.Writer
	Output    io.Writer // the tea.Program's output; nil means os.Stdout
}

// Run opens the UI and returns when the user quits. Task 8 gives it the real
// body once New exists.
func Run(ctx context.Context, d Deps) error {
	_, _ = ctx, d
	return nil
}
