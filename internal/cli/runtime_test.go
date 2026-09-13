package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRuntime(h *harness) *runtime {
	env := h.env
	env.Stdin = nil
	return &runtime{env: env}
}

func TestConnectUsesTokenForSelectedOrigin(t *testing.T) {
	h := newHarness(t)
	r := newRuntime(h)
	svc, _, err := r.connect(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "work", svc.Server.Alias)
	assert.Equal(t, workOrigin, svc.Origin)
	require.Len(t, h.connectOpts, 1)
	assert.Equal(t, "tok", h.connectOpts[0].Token)
	assert.Equal(t, "https://bamboo.example.com", h.connectOpts[0].BaseURL)
}

func TestConnectWithoutTokenIsAuthError(t *testing.T) {
	h := newHarness(t)
	h.kr.m = map[string]string{}
	_, _, err := newRuntime(h).connect(context.Background(), "")
	require.Error(t, err)
	assert.Equal(t, errs.KindAuth, errs.KindOf(err))
	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "bam login work", e.Try)
}

func TestBAMTokenOnlyForBAMURL(t *testing.T) {
	h := newHarness(t)
	h.kr.m = map[string]string{}
	h.vars["BAM_TOKEN"] = "ci-token"
	_, _, err := newRuntime(h).connect(context.Background(), "")
	assert.Equal(t, errs.KindAuth, errs.KindOf(err), "BAM_TOKEN alone never reaches the work alias")

	h.vars["BAM_URL"] = "https://ci.bamboo.example.com"
	svc, _, err := newRuntime(h).connect(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "env", svc.Server.Alias)
	assert.Equal(t, "ci-token", h.connectOpts[len(h.connectOpts)-1].Token)
}

func TestConnectForTargetUsesTargetServer(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(h.env.Paths.MachineConfig), 0o700))
	require.NoError(t, os.WriteFile(h.env.Paths.MachineConfig, []byte(
		"version: 1\nservers:\n  home:\n    url: http://bamboo.lab.example:8085\ntargets:\n  lab-smoke:\n    server: home\n    plan: LAB-SMOKE\n"), 0o644))
	h.kr.m["bam|http://bamboo.lab.example:8085"] = "home-tok"
	svc, _, err := newRuntime(h).connectFor(context.Background(), "lab-smoke")
	require.NoError(t, err)
	assert.Equal(t, "home", svc.Server.Alias)
	assert.Equal(t, "home-tok", h.connectOpts[0].Token)
}

func TestColorFlagValidated(t *testing.T) {
	h := newHarness(t)
	r := newRuntime(h)
	r.flags.color = "sometimes"
	_, err := r.out()
	assert.Equal(t, errs.KindUsage, errs.KindOf(err))
}
