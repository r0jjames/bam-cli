//go:build e2e

// Package e2e runs bam against a real Bamboo. It triggers one build of
// BAM_E2E_PLAN, so point it at a plan that is safe to run (a smoke plan).
//
//	BAM_E2E_URL=http://bamboo.lab.example:8085 BAM_E2E_TOKEN=... BAM_E2E_PLAN=LAB-SMOKE make e2e
package e2e

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r0jjames/bam-cli/internal/cli"
)

func TestSmoke(t *testing.T) {
	url, token, plan := os.Getenv("BAM_E2E_URL"), os.Getenv("BAM_E2E_TOKEN"), os.Getenv("BAM_E2E_PLAN")
	if url == "" || token == "" || plan == "" {
		t.Skip("set BAM_E2E_URL, BAM_E2E_TOKEN and BAM_E2E_PLAN")
	}
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
