package main

import (
	"os"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestStartedByDesktop(t *testing.T) {
	t.Parallel()

	t.Run("no terminal is a desktop's start", func(t *testing.T) {
		t.Parallel()

		file := notTerminal(t)

		assert.True(t, startedByDesktop(file, file, file))
		assert.True(t, startedAsApp([]string{"tarhche"}, file, file, file))
	})

	t.Run("a terminal on any stream is not", func(t *testing.T) {
		t.Parallel()

		file, terminal := notTerminal(t), openTerminal(t)

		assert.False(t, startedByDesktop(terminal, file, file))
		assert.False(t, startedByDesktop(file, terminal, file))
		assert.False(t, startedByDesktop(file, file, terminal))
		assert.False(t, startedAsApp([]string{"tarhche"}, terminal, terminal, terminal))
	})
}

// openTerminal is the terminal end of a new pseudo-terminal.
func openTerminal(t *testing.T) *os.File {
	t.Helper()

	controller, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = controller.Close() })

	require.NoError(t, unix.IoctlSetPointerInt(int(controller.Fd()), unix.TIOCSPTLCK, 0))

	number, err := unix.IoctlGetInt(int(controller.Fd()), unix.TIOCGPTN)
	require.NoError(t, err)

	terminal, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|syscall.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = terminal.Close() })

	return terminal
}
