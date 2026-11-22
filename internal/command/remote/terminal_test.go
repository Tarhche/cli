package remote

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestLocalTerminal(t *testing.T) {
	t.Parallel()

	file, err := os.Create(filepath.Join(t.TempDir(), "not-a-terminal"))
	require.NoError(t, err)

	t.Cleanup(func() { _ = file.Close() })

	t.Run("what is not a file is no terminal", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		assert.False(t, localTerminal{}.IsTerminal(&buffer))

		_, err := localTerminal{}.MakeRaw(&buffer, &buffer)
		assert.ErrorIs(t, err, errNotTerminal)

		_, _, err = localTerminal{}.Size(&buffer)
		assert.ErrorIs(t, err, errNotTerminal)
	})

	t.Run("a file that is not a terminal is none either", func(t *testing.T) {
		t.Parallel()

		assert.False(t, localTerminal{}.IsTerminal(file))

		_, err := localTerminal{}.MakeRaw(file, file)
		assert.Error(t, err)

		_, _, err = localTerminal{}.Size(file)
		assert.Error(t, err)
	})

	t.Run("resizes are told until the context is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		resized := localTerminal{}.Resized(ctx)

		cancel()

		for range resized {
		}
	})
}

// closedSession is a session whose output ends at once, and whose resizes
// fail.
type closedSession struct {
	io.Reader
	resizes int
}

func (s *closedSession) Write(p []byte) (int, error) { return len(p), nil }
func (s *closedSession) Close() error                { return nil }

func (s *closedSession) Resize(uint, uint) error {
	s.resizes++

	return errors.New("cannot resize")
}

func TestAttach(t *testing.T) {
	t.Parallel()

	t.Run("a size the session does not take is told again", func(t *testing.T) {
		t.Parallel()

		out := &bytes.Buffer{}
		terminal := newFakeTerminal(out)
		r := &remote{cli: command.NewCli(strings.NewReader(""), out, out), terminal: terminal}

		session := &closedSession{Reader: strings.NewReader("")}
		r.followSize(context.Background(), session, 0, 0)
		r.followSize(context.Background(), session, 0, 0)

		assert.Equal(t, 2, session.resizes)
	})

	t.Run("a terminal of no size says nothing", func(t *testing.T) {
		t.Parallel()

		out := &bytes.Buffer{}
		terminal := newFakeTerminal(out)
		terminal.rows = 0

		r := &remote{cli: command.NewCli(strings.NewReader(""), out, out), terminal: terminal}

		rows, cols := r.size()
		assert.Zero(t, rows)
		assert.Zero(t, cols)

		session := &closedSession{Reader: strings.NewReader("")}
		require.NoError(t, r.attach(context.Background(), session, attachment{tty: true}))
		assert.Zero(t, session.resizes)
	})

	t.Run("output that cannot be read is the error", func(t *testing.T) {
		t.Parallel()

		out := &bytes.Buffer{}
		r := &remote{cli: command.NewCli(strings.NewReader(""), out, out), terminal: newFakeTerminal()}

		broken := errors.New("lost")
		session := &closedSession{Reader: io.MultiReader(strings.NewReader("partial"), failingReader{err: broken})}

		assert.ErrorIs(t, r.attach(context.Background(), session, attachment{}), broken)
		assert.Equal(t, "partial", out.String())
	})
}

type failingReader struct {
	err error
}

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }
