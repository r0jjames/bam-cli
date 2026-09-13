package bamboo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProbeHappyPath(t *testing.T) {
	c, _ := newTestServer(t, map[string]*route{
		"GET /rest/api/latest/info":                                                            {fixture: "info.json"},
		"GET /rest/api/latest/plan/PROJ-BUILD/variables":                                       {fixture: "plan_variables.json"},
		"GET /rest/api/latest/result/PROJ-BUILD":                                               {fixture: "results.json"},
		"GET /rest/api/latest/result/PROJ-BUILD-482?expand=variables":                          {fixture: "result_variables.json"},
		"GET /rest/api/latest/result/PROJ-BUILD-482?expand=" + buildExpand:                     {fixture: "result_detail.json"},
		"GET /rest/api/latest/result/PROJ-BUILD-481?expand=testResults.failedTests.testResult": {fixture: "failed_tests.json"},
		"GET /rest/api/latest/result/PROJ-BUILD12-JOB1-44?expand=logEntries[0:500]":            {fixture: "log_entries.json"},
	})
	results := c.Probe(ctx, "PROJ-BUILD")
	byName := map[string]ProbeResult{}
	var names []string
	for _, r := range results {
		byName[r.Name] = r
		names = append(names, r.Name)
	}
	assert.Equal(t, []string{"server version", "plan variables", "build variables", "logs", "failed tests", "stop builds"}, names)
	assert.Equal(t, ProbeOK, byName["server version"].Status)
	assert.Equal(t, "9.6.2", byName["server version"].Detail)
	assert.Equal(t, ProbeOK, byName["plan variables"].Status)
	assert.Equal(t, ProbeOK, byName["build variables"].Status)
	assert.Equal(t, ProbeOK, byName["logs"].Status)
	assert.Equal(t, ProbeOK, byName["failed tests"].Status)
	assert.Equal(t, ProbeSkipped, byName["stop builds"].Status)
}

func TestProbeReportsUnsupportedAndErrors(t *testing.T) {
	c, _ := newTestServer(t, map[string]*route{
		"GET /rest/api/latest/info":              {fixture: "info.json"},
		"GET /rest/api/latest/plan/PROJ-BUILD":   {fixture: "plan.json"},
		"GET /rest/api/latest/result/PROJ-BUILD": {body: `{"results":{"size":0,"result":[]}}`},
	})
	results := c.Probe(ctx, "PROJ-BUILD")
	status := map[string]ProbeStatus{}
	for _, r := range results {
		status[r.Name] = r.Status
	}
	assert.Equal(t, ProbeUnsupported, status["plan variables"])
	assert.Equal(t, ProbeSkipped, status["build variables"], "no builds to read from")
	assert.Equal(t, ProbeSkipped, status["logs"])
	assert.Equal(t, ProbeSkipped, status["failed tests"])
}
