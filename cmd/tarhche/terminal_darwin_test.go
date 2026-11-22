package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartedByDesktop(t *testing.T) {
	t.Parallel()

	t.Run("what launchd did not start is no app's start", func(t *testing.T) {
		t.Parallel()

		file := notTerminal(t)

		assert.False(t, startedByDesktop(file, file, file))
	})
}
