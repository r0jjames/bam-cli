package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testEnv() (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return Env{
		Stdin:  strings.NewReader(""),
		Stdout: &out,
		Stderr: &errOut,
		Getenv: func(string) string { return "" },
	}, &out, &errOut
}

func TestVersionPrintsVersion(t *testing.T) {
	env, out, _ := testEnv()
	code := Execute(context.Background(), []string{"version"}, env)
	assert.Equal(t, 0, code)
	assert.Equal(t, "bam dev\n", out.String())
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	env, _, errOut := testEnv()
	code := Execute(context.Background(), []string{"nope"}, env)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut.String(), "unknown command")
}
