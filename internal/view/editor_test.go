package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditorCommand(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	all := map[string]string{"BAM_EDITOR": "nano", "VISUAL": "code --wait", "EDITOR": "vim"}
	assert.Equal(t, "nano", EditorCommand(env(all), "hx", "linux"))
	assert.Equal(t, "hx", EditorCommand(env(map[string]string{"VISUAL": "code --wait", "EDITOR": "vim"}), "hx", "linux"))
	assert.Equal(t, "code --wait", EditorCommand(env(map[string]string{"VISUAL": "code --wait", "EDITOR": "vim"}), "", "linux"))
	assert.Equal(t, "vim", EditorCommand(env(map[string]string{"EDITOR": "vim"}), "", "darwin"))
	assert.Equal(t, "vi", EditorCommand(env(nil), "", "linux"))
	assert.Equal(t, "notepad", EditorCommand(env(nil), "", "windows"))
}
