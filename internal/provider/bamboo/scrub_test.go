package bamboo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrubReplacesHostsUsersAndEmails(t *testing.T) {
	in := `{"link":{"href":"http://ci.home.internal:8085/rest/api/latest/plan/LAB-PROV"},` +
		`"repo":"https://git.home.internal/x.git","reason":"Manual run by <a href=\"http://ci.home.internal:8085/browse/user/rsmith\">Real Name</a>",` +
		`"email":"real.person@home.internal","raw":"agent on ci.home.internal:8085","name":"rsmith"}`
	s := Scrubber{Host: "ci.home.internal:8085", Users: []string{"Real Name", "rsmith"}}
	out := string(s.Scrub([]byte(in)))

	assert.NotContains(t, out, "home.internal")
	assert.NotContains(t, out, "Real Name")
	assert.NotContains(t, out, `"rsmith"`)
	assert.Contains(t, out, "http://bamboo.example.com/rest/api/latest/plan/LAB-PROV")
	assert.Contains(t, out, "jdoe@example.com")
	assert.Contains(t, out, `"name":"jdoe"`)
}

func TestFixtureHostViolations(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"u":"https://bamboo.example.com/x","v":"http://localhost:8085"}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "recorded"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "recorded", "bad.json"), []byte(`{"u":"https://bamboo.corp.internal/x"}`), 0o644))

	v, err := FixtureHostViolations(dir)
	require.NoError(t, err)
	require.Len(t, v, 1)
	assert.Contains(t, v[0], "bad.json")
	assert.Contains(t, v[0], "bamboo.corp.internal")
}

func TestScrubBareHostsAndDifferentCase(t *testing.T) {
	// Test bare hostname without protocol
	in := `{"agent":"ci.home.internal","name":"test"}`
	s := Scrubber{Host: "ci.home.internal", Users: nil}
	out := string(s.Scrub([]byte(in)))
	assert.NotContains(t, out, "ci.home.internal")
	assert.Contains(t, out, "jdoe")

	// Test different case
	in = `{"host":"CI.HOME.INTERNAL","link":"http://CI.HOME.INTERNAL:8085/x"}`
	s = Scrubber{Host: "ci.home.internal:8085", Users: nil}
	out = string(s.Scrub([]byte(in)))
	assert.NotContains(t, out, "CI.HOME.INTERNAL")
	assert.NotContains(t, out, "ci.home.internal")
}

func TestScrubberTerms(t *testing.T) {
	s := Scrubber{Host: "ci.home.internal:8085", Users: []string{"Real Name", "rsmith", "", "Real Name"}}
	terms := s.Terms()
	assert.Contains(t, terms, "ci.home.internal:8085")
	assert.Contains(t, terms, "ci.home.internal")
	assert.Contains(t, terms, "Real Name")
	assert.Contains(t, terms, "rsmith")
	assert.NotContains(t, terms, "")
	// Check no duplicates: convert to map and verify size matches
	termMap := make(map[string]bool)
	for _, term := range terms {
		termMap[term] = true
	}
	assert.Equal(t, len(terms), len(termMap), "terms should have no duplicates, got: %v", terms)
}

func TestFixtureHostViolationsAnyScheme(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"u":"https://bamboo.example.com/x"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ws.json"), []byte(`{"u":"wss://ci.corp.internal/socket"}`), 0o644))

	v, err := FixtureHostViolations(dir)
	require.NoError(t, err)
	require.Len(t, v, 1)
	assert.Contains(t, v[0], "ws.json")
	assert.Contains(t, v[0], "ci.corp.internal")
}

func TestFixtureHostViolationsIPv6Bracket(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{"u":"http://[::1]:8085/x"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"u":"http://[2001:db8::1]:8085/x"}`), 0o644))

	v, err := FixtureHostViolations(dir)
	require.NoError(t, err)
	require.Len(t, v, 1)
	assert.Contains(t, v[0], "bad.json")
	assert.Contains(t, v[0], "[2001:db8::1]")
}

func TestFixtureHostViolationsIPv6BracketIgnoresNonIPCandidates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ok.json"), []byte(
		`{"expand":"logEntries[0:50]","idx":"[1]","letters":"[abc]"}`), 0o644))

	v, err := FixtureHostViolations(dir)
	require.NoError(t, err)
	assert.Empty(t, v, "expand ranges and other bracketed non-IPv6 text must not be flagged: %v", v)
}

func TestFixtureHostViolationsIPAddress(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "has_ip.json"), []byte(`{"host":"10.1.2.3"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "has_localhost.json"), []byte(`{"host":"127.0.0.1"}`), 0o644))

	v, err := FixtureHostViolations(dir)
	require.NoError(t, err)
	// Should find 10.1.2.3 but not 127.0.0.1
	require.Len(t, v, 1)
	assert.Contains(t, v[0], "10.1.2.3")
	assert.NotContains(t, strings.Join(v, ","), "127.0.0.1")
}

func TestFixtureTermViolations(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"user":"RSMITH","data":"some data"}`), 0o644))

	v, err := FixtureTermViolations(dir, []string{"rsmith"})
	require.NoError(t, err)
	require.Len(t, v, 1)
	assert.Contains(t, v[0], "test.json")
	assert.Contains(t, v[0], "rsmith")
}

func TestLoadDenylist(t *testing.T) {
	dir := t.TempDir()
	denylistPath := filepath.Join(dir, "denylist.txt")

	// Test missing file returns nil
	v, err := LoadDenylist(filepath.Join(dir, "missing.txt"))
	require.NoError(t, err)
	require.Nil(t, v)

	// Test loading with comments and blanks
	content := `# Comment
jdoe
ci.home.internal

# Another comment
rsmith`
	require.NoError(t, os.WriteFile(denylistPath, []byte(content), 0o644))

	v, err = LoadDenylist(denylistPath)
	require.NoError(t, err)
	assert.ElementsMatch(t, v, []string{"jdoe", "ci.home.internal", "rsmith"})
}

func TestDenylistPath(t *testing.T) {
	// Test with BAM_FIXTURE_DENYLIST set
	path := DenylistPath(func(key string) string {
		if key == "BAM_FIXTURE_DENYLIST" {
			return "/custom/path"
		}
		return ""
	})
	assert.Equal(t, "/custom/path", path)

	// Test default path
	path = DenylistPath(func(string) string { return "" })
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "bam")
	assert.Contains(t, path, "fixture-denylist.txt")
}

func TestFixtureTermViolationsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"host":"CI.HOME.INTERNAL","name":"RSMITH"}`), 0o644))

	v, err := FixtureTermViolations(dir, []string{"ci.home.internal", "rsmith"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(v))
}

func TestScrubReplacesNamesWithPlaceholders(t *testing.T) {
	s := Scrubber{
		Host:  "bamboo.lab.example:8085",
		Users: []string{"rsmith"},
		Names: map[string]string{"acme-ci": "lab-ci", "ACME": "LAB"},
	}

	out := string(s.Scrub([]byte(`{"key":"ACME-PROV-2","planName":"acme-ci build","repositoryName":"acme-ci","project":"acme"}`)))

	assert.Contains(t, out, `"key":"LAB-PROV-2"`)
	assert.Contains(t, out, `"planName":"lab-ci build"`, "the longer name is replaced before the shorter one")
	assert.Contains(t, out, `"repositoryName":"lab-ci"`)
	assert.Contains(t, out, `"project":"LAB"`, "a differently cased mention is replaced too")
	assert.NotContains(t, strings.ToLower(out), "acme")
}

func TestDenylistTermsIncludeReplacedNames(t *testing.T) {
	s := Scrubber{Host: "bamboo.lab.example:8085", Users: []string{"rsmith"}, Names: map[string]string{"acme-ci": "lab-ci"}}

	terms := s.DenylistTerms()

	assert.Contains(t, terms, "acme-ci")
	assert.Contains(t, terms, "bamboo.lab.example:8085")
	assert.Contains(t, terms, "rsmith")
	assert.NotContains(t, s.Terms(), "acme-ci", "Scrub must not rewrite a project name to jdoe")
}
