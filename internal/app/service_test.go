package app

import (
	"testing"

	"github.com/r0jjames/bam-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestProjectKeysComesFromTheConfiguration(t *testing.T) {
	s := &Service{Cfg: testConfig(t.TempDir()), Server: config.ResolvedServer{Alias: "work"}}
	assert.Equal(t, []string{"PROJ"}, s.ProjectKeys())

	assert.Nil(t, (&Service{}).ProjectKeys(), "no configuration means no scope")
}
