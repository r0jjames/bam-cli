// Package errs defines the error classes bam reports. Every layer returns
// *Error values; only the cli package turns a Kind into an exit code.
package errs

import (
	"errors"
	"fmt"
)

// Kind is the class of an error. It decides the exit code.
type Kind int

const (
	KindInternal Kind = iota // a bug or an unclassified error
	KindUsage                // bad flag, bad argument, failed variable validation
	KindConfig               // configuration files or server selection
	KindAuth                 // missing, rejected or expired token
	KindBamboo               // Bamboo said no, or could not be reached
	KindTimeout              // --timeout elapsed while a build was still running
)

func (k Kind) String() string {
	switch k {
	case KindUsage:
		return "usage"
	case KindConfig:
		return "config"
	case KindAuth:
		return "auth"
	case KindBamboo:
		return "bamboo"
	case KindTimeout:
		return "timeout"
	default:
		return "internal"
	}
}

// Sentinels that adapters wrap so callers can test with errors.Is.
var (
	ErrUnsupported = errors.New("not supported by this Bamboo server")
	ErrNotFound    = errors.New("not found")
)

// Error is what failed, why, and what to try next.
type Error struct {
	Kind Kind
	What string
	Why  string
	Try  string
	Err  error // cause; printed only with --debug
}

// New returns an Error of the given kind.
func New(kind Kind, what string) *Error { return &Error{Kind: kind, What: what} }

// WithWhy sets the reason line.
func (e *Error) WithWhy(why string) *Error { e.Why = why; return e }

// WithTry sets the suggested next command.
func (e *Error) WithTry(try string) *Error { e.Try = try; return e }

// Wrap records the underlying cause.
func (e *Error) Wrap(err error) *Error { e.Err = err; return e }

func (e *Error) Error() string {
	if e.Why == "" {
		return e.What
	}
	return e.What + ": " + e.Why
}

func (e *Error) Unwrap() error { return e.Err }

// Usagef returns a usage error.
func Usagef(format string, a ...any) *Error { return New(KindUsage, fmt.Sprintf(format, a...)) }

// Configf returns a configuration error.
func Configf(format string, a ...any) *Error { return New(KindConfig, fmt.Sprintf(format, a...)) }

// Authf returns an authentication error.
func Authf(format string, a ...any) *Error { return New(KindAuth, fmt.Sprintf(format, a...)) }

// Bamboof returns a Bamboo or network error.
func Bamboof(format string, a ...any) *Error { return New(KindBamboo, fmt.Sprintf(format, a...)) }

// KindOf returns the Kind of the first *Error in err's chain, or KindInternal.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}
