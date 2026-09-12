package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Env is everything the CLI takes from the process. Tests build it by hand.
type Env struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Getenv    func(string) string
	StdoutTTY bool
}

// SystemEnv returns the Env of the running process.
func SystemEnv() Env {
	return Env{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
	}
}
