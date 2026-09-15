package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/r0jjames/bam-cli/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func queuedBuild(key string) provider.Build {
	return provider.Build{Key: key, URL: workOrigin + "/browse/" + key, State: provider.StateQueued}
}

func TestRunDryRunTriggersNothing(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, 0, h.run("run", "provision-lab", "--var", "cluster_name=a", "--dry-run"))
	assert.Contains(t, h.stdout.String(), "Plan     PROJ-PROV  (branch develop, PROJ-PROV12)")
	assert.Contains(t, h.stdout.String(), "cluster_name=a (flag)")
	assert.Contains(t, h.stderr.String(), "dry run: nothing triggered")
	assert.Empty(t, h.fake.Triggered)
}

func TestRunValidationStopsBeforeTrigger(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, 2, h.run("run", "provision-lab"))
	assert.Contains(t, h.stderr.String(), "cluster_name is required by target provision-lab")
	assert.Equal(t, 2, h.run("run", "provision-lab", "--var", "cluster_name=a", "--var", "cluster_type=nomad"))
	assert.Empty(t, h.fake.Triggered)
}

func TestRunWithoutWatch(t *testing.T) {
	h := newHarness(t)
	h.fake.TriggerResult = queuedBuild("PROJ-PROV12-9")
	assert.Equal(t, 0, h.run("run", "provision-lab", "--var", "cluster_name=a", "--from", "last"))
	assert.Contains(t, h.stdout.String(), "✓ Queued   PROJ-PROV12-9")
	assert.Contains(t, h.stdout.String(), "watch: bam watch PROJ-PROV12-9")
	assert.Contains(t, h.stderr.String(), "variables from PROJ-PROV12-8")
	require.Len(t, h.fake.Triggered, 1)
	assert.Equal(t, "PROJ-PROV12", h.fake.Triggered[0].PlanKey)
	assert.Equal(t, "dcos", h.fake.Triggered[0].Variables["cluster_type"], "--from last supplied it")

	assert.Equal(t, 0, h.run("url", "--last"))
	assert.Equal(t, workOrigin+"/browse/PROJ-PROV12-9\n", h.stdout.String())
}

func TestRunJSONWithoutWatch(t *testing.T) {
	h := newHarness(t)
	h.fake.TriggerResult = queuedBuild("PROJ-BUILD-483")
	assert.Equal(t, 0, h.run("build", "run", "build", "--json"))
	var doc map[string]any
	require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &doc))
	assert.Equal(t, "PROJ-BUILD-483", doc["key"])
}

func TestRunWatchExitCodes(t *testing.T) {
	for _, tc := range []struct {
		final provider.State
		code  int
		text  string
	}{
		{provider.StateSuccess, 0, "✓ Success"},
		{provider.StateFailed, 1, "✗ Failed"},
	} {
		h := newHarness(t)
		h.fake.TriggerResult = queuedBuild("PROJ-BUILD-483")
		h.fake.Sequences = map[string][]provider.Build{"PROJ-BUILD-483": {
			{Key: "PROJ-BUILD-483", State: provider.StateRunning},
			{Key: "PROJ-BUILD-483", State: tc.final},
		}}
		assert.Equal(t, tc.code, h.run("run", "build", "--watch"), tc.final)
		assert.Contains(t, h.stdout.String(), "PROJ-BUILD-483 running")
		assert.Contains(t, h.stdout.String(), tc.text)
		assert.Empty(t, h.stderr.String())
	}
}

func TestWatchNDJSON(t *testing.T) {
	h := newHarness(t)
	h.fake.Sequences = map[string][]provider.Build{"PROJ-BUILD-483": {
		{Key: "PROJ-BUILD-483", State: provider.StateRunning},
		{Key: "PROJ-BUILD-483", State: provider.StateSuccess},
	}}
	assert.Equal(t, 0, h.run("watch", "PROJ-BUILD-483", "--json"))
	lines := strings.Split(strings.TrimSpace(h.stdout.String()), "\n")
	var last map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &last))
	assert.Equal(t, "done", last["type"])
}

func TestWatchTimeout(t *testing.T) {
	h := newHarness(t)
	h.fake.Sequences = map[string][]provider.Build{"PROJ-BUILD-483": {{Key: "PROJ-BUILD-483", State: provider.StateRunning}}}
	assert.Equal(t, 6, h.run("watch", "PROJ-BUILD-483", "--timeout", "50ms"))
	assert.Contains(t, h.stderr.String(), "timed out after 50ms")
	assert.Contains(t, h.stderr.String(), "bam watch PROJ-BUILD-483")
	assert.Empty(t, h.fake.Stopped)
}

func TestWatchInterruptLeavesBuildRunning(t *testing.T) {
	h := newHarness(t)
	h.fake.Sequences = map[string][]provider.Build{"PROJ-BUILD-483": {{Key: "PROJ-BUILD-483", State: provider.StateRunning}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Equal(t, 130, h.runCtx(ctx, "watch", "PROJ-BUILD-483"))
	assert.Contains(t, h.stderr.String(), "still running: bam watch PROJ-BUILD-483")
	assert.Empty(t, h.fake.Stopped)
}

func TestLogs(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, 0, h.run("logs", "PROJ-PROV12-8", "--failed"))
	assert.Equal(t, "terraform apply\nError: quota exceeded\n", h.stdout.String())

	assert.Equal(t, 0, h.run("logs", "PROJ-PROV12-8", "--tail", "1"))
	assert.Equal(t, "Error: quota exceeded\n", h.stdout.String())

	assert.Equal(t, 0, h.run("logs", "PROJ-BUILD-482", "--failed"))
	assert.Contains(t, h.stderr.String(), "no failed jobs in PROJ-BUILD-482")

	assert.Equal(t, 0, h.run("logs", "PROJ-BUILD-482", "--failed", "--json"))
	assert.Equal(t, "[]\n", h.stdout.String(), "--json always writes exactly one document, even when empty")

	assert.Equal(t, 2, h.run("logs", "--last"))
	assert.Equal(t, 0, h.run("logs", "PROJ-PROV12-8", "--follow"))
	assert.Contains(t, h.stdout.String(), "Error: quota exceeded")

	assert.Equal(t, 0, h.run("logs", "PROJ-PROV12-8", "--json"))
	assert.Contains(t, h.stdout.String(), `"job": "PROJ-PROV12-TF-8"`)
}

func TestCancel(t *testing.T) {
	h := newHarness(t)
	h.fake.Sequences = map[string][]provider.Build{"PROJ-BUILD-483": {{Key: "PROJ-BUILD-483", State: provider.StateRunning}}}
	assert.Equal(t, 0, h.run("build", "cancel", "PROJ-BUILD-483"))
	assert.Contains(t, h.stdout.String(), "stopping PROJ-BUILD-483")
	assert.Equal(t, []string{"PROJ-BUILD-483"}, h.fake.Stopped)

	assert.Equal(t, 0, h.run("build", "cancel", "PROJ-BUILD-482"))
	assert.Contains(t, h.stderr.String(), "PROJ-BUILD-482 already finished (success)")
	assert.Len(t, h.fake.Stopped, 1)

	assert.Equal(t, 0, h.run("build", "cancel", "PROJ-BUILD-482", "--json"))
	assert.Empty(t, h.stderr.String(), "the human-only note is not printed in --json mode")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(h.stdout.Bytes(), &doc))
	assert.Equal(t, "PROJ-BUILD-482", doc["key"])
	assert.Equal(t, false, doc["stopped"])
	assert.Equal(t, "success", doc["state"])
}

func TestOpenAndURL(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, 0, h.run("open", "provision-lab"))
	assert.Equal(t, []string{workOrigin + "/browse/PROJ-PROV"}, h.opened)
	assert.Equal(t, 0, h.run("url", "PROJ-BUILD-482"))
	assert.Equal(t, workOrigin+"/browse/PROJ-BUILD-482\n", h.stdout.String())
	assert.Equal(t, 0, h.run("url", "PROJ"))
	assert.Equal(t, 0, h.run("url", "PROJ-BUILD-JOB1-482"))
	assert.Equal(t, 2, h.run("url", "not a key"))
	assert.Equal(t, 2, h.run("open"))

	h.env.OpenBrowser = func(string) error { return errors.New("no display") }
	assert.Equal(t, 0, h.run("open", "PROJ-BUILD"))
	assert.Equal(t, workOrigin+"/browse/PROJ-BUILD\n", h.stdout.String())
	assert.Contains(t, h.stderr.String(), "could not open a browser")
}

func TestVarCompletionUsesTargetOptions(t *testing.T) {
	h := newHarness(t)
	h.run("__complete", "run", "provision-lab", "--var", "cluster_type=")
	assert.Contains(t, h.stdout.String(), "cluster_type=k8s\n")
	assert.Contains(t, h.stdout.String(), "cluster_type=dcos\n")

	h.run("__complete", "run", "prov")
	assert.Contains(t, h.stdout.String(), "provision-lab\n")
}

func TestHelpListsShortcuts(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, 0, h.run("--help"))
	assert.Contains(t, h.stdout.String(), "Shortcuts:")
	assert.Regexp(t, `run\s+Trigger a plan or target`, h.stdout.String())
}
