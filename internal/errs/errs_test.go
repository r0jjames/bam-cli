package errs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorMessageJoinsWhatAndWhy(t *testing.T) {
	e := New(KindBamboo, "could not start build of PROJ-BUILD").
		WithWhy(`branch "feat/foo" not found on server "work"`).
		WithTry("bam plan branches PROJ-BUILD")
	assert.Equal(t, `could not start build of PROJ-BUILD: branch "feat/foo" not found on server "work"`, e.Error())
	assert.Equal(t, "bam plan branches PROJ-BUILD", e.Try)
}

func TestKindOfFindsWrappedError(t *testing.T) {
	inner := Authf("401 from server %q", "work")
	wrapped := fmt.Errorf("listing plans: %w", inner)
	assert.Equal(t, KindAuth, KindOf(wrapped))
	assert.Equal(t, KindInternal, KindOf(errors.New("plain")))
	assert.Equal(t, KindInternal, KindOf(nil))
}

func TestWrapKeepsCauseForErrorsIs(t *testing.T) {
	e := Bamboof("plan %s not found", "PROJ-X").Wrap(ErrNotFound)
	assert.True(t, errors.Is(e, ErrNotFound))
	assert.Equal(t, KindBamboo, KindOf(e))
}

func TestKindString(t *testing.T) {
	assert.Equal(t, "usage", KindUsage.String())
	assert.Equal(t, "timeout", KindTimeout.String())
}
