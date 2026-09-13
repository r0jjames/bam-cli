package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"

	"github.com/adrg/xdg"
	"github.com/pkg/browser"
	"github.com/r0jjames/bam-cli/internal/app"
	"github.com/r0jjames/bam-cli/internal/credential"
	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/r0jjames/bam-cli/internal/provider/bamboo"
	"golang.org/x/term"
)

// Backend is a server connection: the neutral Provider plus doctor's probe.
type Backend interface {
	provider.Provider
	Probe(ctx context.Context, planKey string) []bamboo.ProbeResult
}

// Paths are bam's files outside the repository.
type Paths struct {
	MachineConfig string
	Credentials   string
	Capabilities  string
	State         string
}

// Env is everything the CLI takes from the process. Tests build it by hand.
type Env struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Getenv      func(string) string
	StdoutTTY   bool
	StdinTTY    bool
	Width       func() int
	WorkDir     string
	Home        string
	Paths       Paths
	Clock       app.Clock
	Keyring     credential.Keyring
	Connect     func(bamboo.Options) (Backend, error)
	OpenBrowser func(url string) error
	RunPager    func(cmd string, r io.Reader) error
	ReadSecret  func() (string, error)
	GOOS        string
}

// SystemEnv returns the Env of the running process.
func SystemEnv() Env {
	wd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	machine := os.Getenv("BAM_CONFIG")
	if machine == "" {
		machine = filepath.Join(xdg.ConfigHome, "bam", "config.yaml")
	}
	out := int(os.Stdout.Fd())
	return Env{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		StdoutTTY: term.IsTerminal(out),
		StdinTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		Width: func() int {
			if w, _, err := term.GetSize(out); err == nil && w > 0 {
				return w
			}
			if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
				return c
			}
			return 80
		},
		WorkDir: wd,
		Home:    home,
		Paths: Paths{
			MachineConfig: machine,
			Credentials:   filepath.Join(xdg.ConfigHome, "bam", "credentials.yaml"),
			Capabilities:  filepath.Join(xdg.CacheHome, "bam", "capabilities.json"),
			State:         filepath.Join(xdg.StateHome, "bam", "state.json"),
		},
		Clock:   app.SystemClock{},
		Keyring: credential.SystemKeyring{},
		Connect: func(o bamboo.Options) (Backend, error) {
			c, err := bamboo.New(o)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		OpenBrowser: browser.OpenURL,
		RunPager:    runPager,
		ReadSecret: func() (string, error) {
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			return string(b), err
		},
		GOOS: goruntime.GOOS,
	}
}
