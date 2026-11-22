package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	t.Parallel()

	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer

		status := run(context.Background(), append([]string{"tarhche"}, args...), strings.NewReader(""), &out, &errOut)

		return status, out.String(), errOut.String()
	}

	t.Run("--version prints the version", func(t *testing.T) {
		t.Parallel()

		status, out, _ := run("--version")

		assert.Equal(t, 0, status)
		assert.Equal(t, "Tarhche CLI version dev\n", out)
	})

	t.Run("--help prints the help", func(t *testing.T) {
		t.Parallel()

		status, out, _ := run("--help")

		assert.Equal(t, 0, status)
		assert.Contains(t, out, "Usage:")
		assert.Contains(t, out, "--desktop")
	})

	t.Run("an unknown command is a usage error", func(t *testing.T) {
		t.Parallel()

		status, _, errOut := run("bogus")

		assert.Equal(t, 2, status)
		assert.Contains(t, errOut, `"bogus" is not a command`)
	})

	t.Run("--desktop opens the desktop app", func(t *testing.T) {
		t.Parallel()

		status, _, errOut := run("--desktop")

		assert.Equal(t, 1, status)
		assert.Contains(t, errOut, "desktop app")
	})
}
