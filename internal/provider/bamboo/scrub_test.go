package bamboo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScrubReplacesHostsUsersAndEmails(t *testing.T) {
	in := `{"link":{"href":"http://ci.home.internal:8085/rest/api/latest/plan/LAB-PROV"},` +
		`"repo":"https://git.home.internal/x.git","reason":"Manual run by <a href=\"http://ci.home.internal:8085/browse/user/rjc\">Real Name</a>",` +
		`"email":"real.person@home.internal","raw":"agent on ci.home.internal:8085","name":"rjc"}`
	s := Scrubber{Host: "ci.home.internal:8085", Users: []string{"Real Name", "rjc"}}
	out := string(s.Scrub([]byte(in)))

	assert.NotContains(t, out, "home.internal")
	assert.NotContains(t, out, "Real Name")
	assert.NotContains(t, out, `"rjc"`)
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
