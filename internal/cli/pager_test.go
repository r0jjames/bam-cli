package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capture runs f with os.Stdout replaced by a pipe and returns what it wrote.
func capture(t *testing.T, f func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stdout
	os.Stdout = w
	err = f()
	os.Stdout = saved
	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, copyErr := buf.ReadFrom(r)
	require.NoError(t, copyErr)
	require.NoError(t, err)
	return buf.String()
}

func TestPagerKeepsArgumentsOfACatCommand(t *testing.T) {
	out := capture(t, func() error { return runPager("cat -n", strings.NewReader("one\n")) })
	assert.Contains(t, out, "1\tone", "PAGER='cat -n' must run cat with its arguments")
}

func TestPagerCopiesWhenEmptyOrPlainCat(t *testing.T) {
	assert.Equal(t, "x\n", capture(t, func() error { return runPager("", strings.NewReader("x\n")) }))
	assert.Equal(t, "x\n", capture(t, func() error { return runPager("cat", strings.NewReader("x\n")) }))
}
