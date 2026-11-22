package content

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestTerminalPrompter(t *testing.T) {
	t.Parallel()

	t.Run("no terminal to read at", func(t *testing.T) {
		t.Parallel()

		_, err := terminalPrompter(command.NewCli(strings.NewReader("typed"), &bytes.Buffer{}, &bytes.Buffer{}))("Password: ")

		assert.ErrorIs(t, err, errNoTerminal)
	})

	t.Run("a file that is no terminal", func(t *testing.T) {
		t.Parallel()

		file, err := os.Create(filepath.Join(t.TempDir(), "in"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = file.Close() })

		_, err = terminalPrompter(command.NewCli(file, &bytes.Buffer{}, file))("Password: ")

		assert.ErrorIs(t, err, errNoTerminal)
	})
}

func TestReadLines(t *testing.T) {
	t.Parallel()

	t.Run("each without its line ending, the last ending the input", func(t *testing.T) {
		t.Parallel()

		lines, err := readLines(strings.NewReader("one\r\ntwo"), 2)

		require.NoError(t, err)
		assert.Equal(t, []string{"one", "two"}, lines)
	})

	t.Run("an input that ends before them", func(t *testing.T) {
		t.Parallel()

		_, err := readLines(strings.NewReader("one\n"), 2)

		assert.EqualError(t, err, "the standard input ends before its line 2")
	})

	t.Run("an input that cannot be read", func(t *testing.T) {
		t.Parallel()

		_, err := readLines(failingReader{}, 1)

		assert.EqualError(t, err, "reading the standard input: broken")
	})
}
