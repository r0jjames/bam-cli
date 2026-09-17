//go:build e2e

// Package e2e runs bam against a real Bamboo. The server and its token come
// from your bam configuration (the alias in ~/.config/bam/config.yaml and
// the token stored by "bam login"); the plan comes from a flag or from a
// configured target. The suite triggers one build, so name a plan that is
// safe to run (a smoke plan).
//
//	make e2e ARGS='-target smoke'
//	make e2e ARGS='-server lab -plan LAB-SMOKE'
package e2e

import (
	"bytes"
	"context"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r0jjames/bam-cli/internal/cli"
	"github.com/r0jjames/bam-cli/internal/toolcfg"
)

var (
	serverAlias = flag.String("server", "", "server alias (default: the one bam would use in this directory)")
	targetName  = flag.String("target", "", "configured target to take the plan key from")
	planKey     = flag.String("plan", "", "plan key that is safe to run (wins over -target)")
)

func TestSmoke(t *testing.T) {
	if *planKey == "" && *targetName == "" {
		t.Skip("pass -plan KEY or -target NAME, e.g. make e2e ARGS='-target smoke'")
	}
	opts, err := toolcfg.SystemOptions()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	cfg, err := toolcfg.Load(opts)
	if err != nil {
		t.Fatalf("read bam config: %v", err)
	}
	plan, targetServer := *planKey, ""
	if plan == "" {
		plan, targetServer, err = cfg.PlanFor(*targetName)
		if err != nil {
			t.Fatalf("target %s: %v", *targetName, err)
		}
	}
	server, err := cfg.ServerFor(*serverAlias, targetServer)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	t.Logf("e2e against %s (%s), plan %s", server.Alias, server.URL, plan)
	// The suite runs bam in a throwaway home, so the resolved server is
	// handed to it as the ad-hoc BAM_URL server rather than through the
	// developer's own config files.
	url, token := server.URL, server.Token
	dir := t.TempDir()
	run := func(args ...string) (int, string, string) {
		t.Helper()
		env := cli.SystemEnv()
		var out, errOut bytes.Buffer
		env.Stdout, env.Stderr, env.StdoutTTY, env.StdinTTY = &out, &errOut, false, false
		env.WorkDir, env.Home = dir, dir
		env.Paths = cli.Paths{
			MachineConfig: filepath.Join(dir, "config.yaml"),
			Credentials:   filepath.Join(dir, "credentials.yaml"),
			Capabilities:  filepath.Join(dir, "capabilities.json"),
			State:         filepath.Join(dir, "state.json"),
		}
		env.Getenv = func(k string) string {
			switch k {
			case "BAM_URL":
				return url
			case "BAM_TOKEN":
				return token
			}
			return ""
		}
		code := cli.Execute(context.Background(), args, env)
		t.Logf("bam %s → %d\n%s%s", strings.Join(args, " "), code, out.String(), errOut.String())
		return code, out.String(), errOut.String()
	}

	if code, out, _ := run("whoami"); code != 0 || !strings.Contains(out, "env") {
		t.Fatalf("whoami failed")
	}
	if code, _, _ := run("doctor", "--plan", plan); code != 0 {
		t.Fatalf("doctor failed")
	}
	for _, args := range [][]string{{"plan", "show", plan}, {"plan", "vars", plan}, {"build", "list", plan}, {"build", "list", plan, "--json"}} {
		if code, _, _ := run(args...); code != 0 {
			t.Fatalf("%v failed", args)
		}
	}
	code, _, _ := run("run", plan, "--watch", "--timeout", "20m")
	if code != 0 && code != 1 {
		t.Fatalf("run --watch exited %d; want 0 or 1 (the build result)", code)
	}
	if code, out, _ := run("url", "--last"); code != 0 || !strings.Contains(out, "/browse/") {
		t.Fatalf("url --last failed")
	}
	if code, _, _ := run("logs", "--last"); code != 0 {
		t.Fatalf("logs --last failed")
	}
}
