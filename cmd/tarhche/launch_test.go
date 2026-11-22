package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "nothing but the binary", args: []string{"tarhche"}, want: []string{"tarhche"}},
		{name: "a command and its flags", args: []string{"tarhche", "ps", "-a"}, want: []string{"tarhche", "ps", "-a"}},
		{name: "the process serial number macOS adds", args: []string{"tarhche", "-psn_0_12345"}, want: []string{"tarhche"}},
		{name: "a binary named like one", args: []string{"-psn_0_1", "ps"}, want: []string{"-psn_0_1", "ps"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, commandLine(test.args))
		})
	}
}

func TestStartedAsApp(t *testing.T) {
	t.Parallel()

	t.Run("something to run is never an app's start", func(t *testing.T) {
		t.Parallel()

		file := notTerminal(t)

		assert.False(t, startedAsApp([]string{"tarhche", "ps"}, file, file, file))
	})
}

func TestDesktopUnavailable(t *testing.T) {
	t.Parallel()

	noDisplay := unavailableError{err: errors.New("no display")}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nothing", err: nil, want: false},
		{name: "a desktop app that failed", err: errors.New("boom"), want: false},
		{name: "one that cannot be opened here", err: noDisplay, want: true},
		{name: "wrapped", err: fmt.Errorf("opening: %w", noDisplay), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, desktopUnavailable(test.err))
		})
	}

	t.Run("says what it marks", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "no display", noDisplay.Error())
		assert.ErrorIs(t, fmt.Errorf("opening: %w", noDisplay), noDisplay.err)
	})
}

// notTerminal is a file that is no terminal.
func notTerminal(t *testing.T) *os.File {
	t.Helper()

	file, err := os.Create(filepath.Join(t.TempDir(), "stream"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	return file
}
