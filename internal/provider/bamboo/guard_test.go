package bamboo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFixtureGuard fails when any fixture names a host outside the allowlist.
// make check-fixtures runs only this test; CI runs it with everything else.
func TestFixtureGuard(t *testing.T) {
	v, err := FixtureHostViolations("testdata")
	require.NoError(t, err)
	require.Empty(t, v, "fixtures must use placeholder hosts:\n%s", strings.Join(v, "\n"))
}
