package app

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppStaysUIReady enforces spec §8.5: no cobra, no bamboo adapter, no
// direct terminal I/O in package app.
func TestAppStaysUIReady(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		parsed, err := parser.ParseFile(token.NewFileSet(), f, src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			assert.NotContains(t, path, "cobra", f)
			assert.NotContains(t, path, "provider/bamboo", f)
		}
		text := string(src)
		for _, banned := range []string{"os.Stdout", "os.Stderr", "os.Stdin", "fmt.Print"} {
			assert.NotContains(t, text, banned, f)
		}
	}
}
