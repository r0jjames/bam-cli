package toolcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memKeyring map[string]string

func (m memKeyring) Get(service, user string) (string, error) {
	v, ok := m[service+"\x00"+user]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}
func (m memKeyring) Set(service, user, pw string) error { m[service+"\x00"+user] = pw; return nil }
func (m memKeyring) Delete(service, user string) error {
	delete(m, service+"\x00"+user)
	return nil
}

func write(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func opts(t *testing.T, dir string, ring memKeyring) Options {
	t.Helper()
	return Options{
		WorkDir:     dir,
		Home:        dir,
		MachineFile: filepath.Join(dir, "config.yaml"),
		Credentials: filepath.Join(dir, "credentials.yaml"),
		Getenv:      func(string) string { return "" },
		Keyring:     ring,
	}
}

func TestServerReadsURLFromConfigAndTokenFromKeychain(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085\ndefault_server: lab\n")
	ring := memKeyring{"bam\x00http://bamboo.lab.example:8085": "tok-1"}

	l, err := Load(opts(t, dir, ring))
	require.NoError(t, err)
	s, err := l.Server("")
	require.NoError(t, err)

	assert.Equal(t, "lab", s.Alias)
	assert.Equal(t, "http://bamboo.lab.example:8085", s.URL)
	assert.Equal(t, "tok-1", s.Token)
}

func TestServerTrimsTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085/\n")
	ring := memKeyring{"bam\x00http://bamboo.lab.example:8085": "tok-1"}

	l, err := Load(opts(t, dir, ring))
	require.NoError(t, err)
	s, err := l.Server("lab")
	require.NoError(t, err)
	assert.Equal(t, "http://bamboo.lab.example:8085", s.URL)
}

func TestServerWithoutTokenSaysHowToLogIn(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085\n")

	l, err := Load(opts(t, dir, memKeyring{}))
	require.NoError(t, err)
	_, err = l.Server("lab")

	require.Error(t, err)
	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, errs.KindAuth, e.Kind)
	assert.Contains(t, e.Try, "bam login lab")
}

func TestUnknownAliasIsAConfigError(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085\n")

	l, err := Load(opts(t, dir, memKeyring{}))
	require.NoError(t, err)
	_, err = l.Server("nope")

	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, errs.KindConfig, e.Kind)
}

func TestPlanForTargetReadsProjectFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085\n")
	write(t, filepath.Join(dir, ".bam.yaml"), "version: 1\ndefault_server: lab\ntargets:\n  smoke:\n    plan: LAB-SMOKE\n")

	l, err := Load(opts(t, dir, memKeyring{}))
	require.NoError(t, err)

	plan, server, err := l.PlanFor("smoke")
	require.NoError(t, err)
	assert.Equal(t, "LAB-SMOKE", plan)
	assert.Equal(t, "", server, "the target names no server of its own")
}

func TestPlanForUnknownTargetListsTargets(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "config.yaml"), "version: 1\nservers:\n  lab:\n    url: http://bamboo.lab.example:8085\n")
	write(t, filepath.Join(dir, ".bam.yaml"), "version: 1\ndefault_server: lab\ntargets:\n  smoke:\n    plan: LAB-SMOKE\n")

	l, err := Load(opts(t, dir, memKeyring{}))
	require.NoError(t, err)
	_, _, err = l.PlanFor("nope")

	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, errs.KindConfig, e.Kind)
	assert.Contains(t, e.Why, "smoke")
}
