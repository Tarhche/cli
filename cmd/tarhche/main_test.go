package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/command"
)

func TestRun(t *testing.T) {
	t.Parallel()

	type outcome struct {
		status      int
		out, errOut string
		opened      int32
	}

	run := func(started launch, opening error, args ...string) outcome {
		var (
			out, errOut bytes.Buffer
			opened      atomic.Int32
		)

		started.openDesktop = func(context.Context, *command.Cli) error {
			opened.Add(1)

			return opening
		}

		status := run(context.Background(), append([]string{"tarhche"}, args...), strings.NewReader(""), &out, &errOut, started)

		return outcome{status: status, out: out.String(), errOut: errOut.String(), opened: opened.Load()}
	}

	noDisplay := unavailableError{err: errors.New("the desktop app needs a display to open on")}

	t.Run("--version prints the version", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, nil, "--version")

		assert.Equal(t, 0, got.status)
		assert.Equal(t, "Tarhche CLI version dev\n", got.out)
		assert.Zero(t, got.opened)
	})

	t.Run("--help prints the help", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, nil, "--help")

		assert.Equal(t, 0, got.status)
		assert.Contains(t, got.out, "Usage:")
		assert.Contains(t, got.out, "--desktop")
	})

	t.Run("an unknown command is a usage error", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, nil, "bogus")

		assert.Equal(t, 2, got.status)
		assert.Contains(t, got.errOut, `"bogus" is not a command`)
	})

	t.Run("--desktop opens the desktop app", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, nil, "--desktop")

		assert.Equal(t, 0, got.status)
		assert.Equal(t, int32(1), got.opened)
		assert.Empty(t, got.out)
	})

	t.Run("--desktop says why no desktop app opens here", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, noDisplay, "--desktop")

		assert.Equal(t, 1, got.status)
		assert.Contains(t, got.errOut, "needs a display")
		assert.Empty(t, got.out)
	})

	t.Run("started as an app, it opens the desktop app", func(t *testing.T) {
		t.Parallel()

		got := run(launch{asApp: true}, nil)

		assert.Equal(t, 0, got.status)
		assert.Equal(t, int32(1), got.opened)
		assert.Empty(t, got.out)
	})

	t.Run("started as an app where no desktop app opens, it is the command line's usage", func(t *testing.T) {
		t.Parallel()

		got := run(launch{asApp: true}, noDisplay)

		assert.Equal(t, 2, got.status)
		assert.Equal(t, int32(1), got.opened)
		assert.Contains(t, got.errOut, "Usage:")
		assert.NotContains(t, got.errOut, "needs a display")
	})

	t.Run("started as an app, a desktop app that fails says why", func(t *testing.T) {
		t.Parallel()

		got := run(launch{asApp: true}, errors.New("the web view could not start"))

		assert.Equal(t, 1, got.status)
		assert.Contains(t, got.errOut, "the web view could not start")
		assert.Empty(t, got.out)
	})

	t.Run("started from a terminal, nothing to run is the command line's usage", func(t *testing.T) {
		t.Parallel()

		got := run(launch{}, nil)

		assert.Equal(t, 2, got.status)
		assert.Zero(t, got.opened)
		assert.Contains(t, got.errOut, "Usage:")
	})
}
