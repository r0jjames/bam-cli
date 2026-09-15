package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrubVariableValuesReplacesEveryValueButMasked(t *testing.T) {
	in := []byte(`[{"name":"cluster_type","value":"k8s"},{"name":"compute_nodes","value":"2"},{"name":"db_password","value":"********"}]`)
	out, err := scrubVariableValues(in)
	require.NoError(t, err)

	var items []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal(out, &items))
	require.Len(t, items, 3)
	assert.Equal(t, "value-1", items[0].Value)
	assert.Equal(t, "value-2", items[1].Value)
	assert.Equal(t, "********", items[2].Value, "a masked value is left as is")
}

func TestScrubVariableValuesHandlesNestedEnvelope(t *testing.T) {
	in := []byte(`{"key":"PROJ-BUILD-482","buildNumber":482,"variables":{"size":2,"start-index":0,"max-result":2,"variable":[` +
		`{"key":"cluster_type","value":"dcos"},{"key":"cluster_name","value":"beta"}]}}`)
	out, err := scrubVariableValues(in)
	require.NoError(t, err)

	var doc struct {
		Variables struct {
			Variable []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"variable"`
		} `json:"variables"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.Variables.Variable, 2)
	assert.Equal(t, "value-1", doc.Variables.Variable[0].Value)
	assert.Equal(t, "value-2", doc.Variables.Variable[1].Value)
	assert.Equal(t, "cluster_type", doc.Variables.Variable[0].Key, "name/key is untouched")
}

func TestScrubLogEntriesTruncatesAndReplacesText(t *testing.T) {
	in := []byte(`{"logEntries":{"size":7,"start-index":0,"max-result":50,"logEntry":[` +
		`{"date":1,"log":"one","unstyledLog":"one"},` +
		`{"date":2,"log":"two","unstyledLog":"two"},` +
		`{"date":3,"log":"three","unstyledLog":"three"},` +
		`{"date":4,"log":"four","unstyledLog":"four"},` +
		`{"date":5,"log":"five","unstyledLog":"five"},` +
		`{"date":6,"log":"six","unstyledLog":"six"},` +
		`{"date":7,"log":"seven","unstyledLog":"seven"}]}}`)
	out, err := scrubLogEntries(in)
	require.NoError(t, err)

	var doc struct {
		LogEntries struct {
			Size     int `json:"size"`
			LogEntry []struct {
				Log         string `json:"log"`
				UnstyledLog string `json:"unstyledLog"`
			} `json:"logEntry"`
		} `json:"logEntries"`
	}
	require.NoError(t, json.Unmarshal(out, &doc))
	require.Len(t, doc.LogEntries.LogEntry, 5, "at most the first 5 entries are kept")
	assert.Equal(t, 5, doc.LogEntries.Size)
	for i, e := range doc.LogEntries.LogEntry {
		want := "log line " + string(rune('1'+i))
		assert.Equal(t, want, e.Log)
		assert.Equal(t, want, e.UnstyledLog)
	}
}

func TestScrubLogDownloadTruncatesAndReplacesMessage(t *testing.T) {
	in := []byte("simple\t01-Jan-2026 00:00:01\tfirst message\n" +
		"simple\t01-Jan-2026 00:00:02\tsecond message\n" +
		"simple\t01-Jan-2026 00:00:03\tthird message\n" +
		"simple\t01-Jan-2026 00:00:04\tfourth message\n" +
		"simple\t01-Jan-2026 00:00:05\tfifth message\n" +
		"simple\t01-Jan-2026 00:00:06\tsixth message\n")
	out := scrubLogDownload(in)
	lines := splitLines(string(out))
	require.Len(t, lines, 5, "at most the first 5 lines are kept")
	assert.Equal(t, "simple\t01-Jan-2026 00:00:01\tlog line 1", lines[0])
	assert.Equal(t, "simple\t01-Jan-2026 00:00:05\tlog line 5", lines[4])
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
