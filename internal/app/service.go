// Package app holds bam's use cases. It returns values and emits events; it
// never prints, never reads stdin, and never imports cobra, so the commands
// and the terminal UI share it.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
)

// Clock is injected so polling is testable without sleeping.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// SystemClock is the real clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time                         { return time.Now() }
func (SystemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Service runs use cases against one server.
type Service struct {
	P      provider.Provider
	Cfg    *config.Config
	Server config.ResolvedServer
	Origin string // credential.Origin(Server.URL)
	Clock  Clock
	State  *StateStore
	Getenv func(string) string
}

// Build fetches a build. When the last build recorded for this repository no
// longer exists on the server, the record is removed and the error says so.
func (s *Service) Build(ctx context.Context, key string) (provider.Build, error) {
	b, err := s.P.GetBuild(ctx, key)
	if err != nil && errors.Is(err, errs.ErrNotFound) {
		if rec, ok, _ := s.State.Last(s.Cfg.RepoRoot); ok && rec.BuildKey == key {
			_ = s.State.Forget(s.Cfg.RepoRoot)
			return b, errs.Bamboof("the last build %s no longer exists on %s", key, s.Server.Alias).
				WithWhy("it was removed on the server, so bam forgot it").Wrap(errs.ErrNotFound)
		}
	}
	return b, err
}
