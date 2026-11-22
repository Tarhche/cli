package desktop_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
)

func TestTerminals(t *testing.T) {
	t.Parallel()

	shell := desktop.TerminalRequest{Target: desktop.Target{VM: "v-1"}, Size: desktop.TerminalSize{Rows: 24, Cols: 80}}

	// open is a terminal service whose opener opens terminal for shell.
	open := func(t *testing.T, terminal desktop.Terminal) (*desktop.Terminals, *mock.TerminalOpener, *mock.Emitter, *desktop.Workspace) {
		t.Helper()

		w, _ := newWorkspace(t, nil)
		opener := &mock.TerminalOpener{}
		emitter := mock.NewEmitter()

		if terminal != nil {
			opener.On("Open", testify.Anything, w.Cli(), shell).Return(terminal, nil)
		}

		t.Cleanup(func() { opener.AssertExpectations(t) })

		return desktop.NewTerminals(context.Background(), w, opener, emitter), opener, emitter, w
	}

	t.Run("what a terminal says is told, and then that it ended", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminals, _, emitter, _ := open(t, terminal)

		require.NoError(t, terminals.Open("t-1", shell))

		terminal.Say([]byte("$ "))
		terminal.Say([]byte("ls\r\n"))
		terminal.End(io.EOF)

		assert.Equal(t, mock.Event{Name: desktop.EventTerminalOutput, Data: desktop.TerminalOutput{ID: "t-1", Data: []byte("$ ")}}, emitter.Next(t))
		assert.Equal(t, mock.Event{Name: desktop.EventTerminalOutput, Data: desktop.TerminalOutput{ID: "t-1", Data: []byte("ls\r\n")}}, emitter.Next(t))
		assert.Equal(t, mock.Event{Name: desktop.EventTerminalExit, Data: desktop.TerminalExit{ID: "t-1"}}, emitter.Next(t))
		assert.True(t, terminal.IsClosed())
		assert.Error(t, terminals.Write("t-1", []byte("x")), "a terminal that ended is forgotten")
	})

	t.Run("a terminal is opened in the context the app talks to, for as long as the app", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		opener := &mock.TerminalOpener{}
		ctx, cancel := context.WithCancel(context.Background())
		terminal := mock.NewTerminal()

		var opened context.Context
		opener.On("Open", testify.Anything, w.Cli(), shell).Run(func(args testify.Arguments) {
			opened = args.Get(0).(context.Context)
		}).Return(terminal, nil)

		terminals := desktop.NewTerminals(ctx, w, opener, mock.NewEmitter())
		require.NoError(t, terminals.Open("t-1", shell))

		require.NoError(t, opened.Err())
		cancel()
		<-opened.Done()
	})

	t.Run("what ran on a terminal ending with a status says it", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminals, _, emitter, _ := open(t, terminal)
		require.NoError(t, terminals.Open("t-1", shell))

		terminal.End(command.StatusError{Code: 3, Message: "exit status 3"})

		assert.Equal(t, mock.Event{Name: desktop.EventTerminalExit, Data: desktop.TerminalExit{ID: "t-1", Code: 3, Error: "exit status 3"}}, emitter.Next(t))
	})

	t.Run("a terminal that broke says why", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminals, _, emitter, _ := open(t, terminal)
		require.NoError(t, terminals.Open("t-1", shell))

		terminal.End(errors.New("the connection was lost"))

		assert.Equal(t, mock.Event{Name: desktop.EventTerminalExit, Data: desktop.TerminalExit{ID: "t-1", Code: -1, Error: "the connection was lost"}}, emitter.Next(t))
	})

	t.Run("what is typed and the size it is given reach the terminal", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminals, _, _, _ := open(t, terminal)
		require.NoError(t, terminals.Open("t-1", shell))

		require.NoError(t, terminals.Write("t-1", []byte("ls")))
		require.NoError(t, terminals.Write("t-1", []byte("\r")))
		require.NoError(t, terminals.Resize("t-1", desktop.TerminalSize{Rows: 40, Cols: 120}))

		assert.Equal(t, "ls\r", terminal.Input())
		assert.Equal(t, []desktop.TerminalSize{{Rows: 40, Cols: 120}}, terminal.Sizes())
	})

	t.Run("what a terminal refuses is an error", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminal.WriteErr = errors.New("write refused")
		terminal.ResizeErr = errors.New("resize refused")
		terminals, _, _, _ := open(t, terminal)
		require.NoError(t, terminals.Open("t-1", shell))

		assert.ErrorIs(t, terminals.Write("t-1", []byte("ls")), terminal.WriteErr)
		assert.ErrorIs(t, terminals.Resize("t-1", desktop.TerminalSize{Rows: 1, Cols: 1}), terminal.ResizeErr)
	})

	t.Run("a terminal that is not open is not written to nor resized", func(t *testing.T) {
		t.Parallel()

		terminals, _, _, _ := open(t, nil)

		assert.EqualError(t, terminals.Write("t-9", []byte("ls")), `there is no terminal "t-9"`)
		assert.EqualError(t, terminals.Resize("t-9", desktop.TerminalSize{}), `there is no terminal "t-9"`)
		assert.NoError(t, terminals.Close("t-9"))
	})

	t.Run("a terminal closed ends well", func(t *testing.T) {
		t.Parallel()

		terminal := mock.NewTerminal()
		terminals, _, emitter, _ := open(t, terminal)
		require.NoError(t, terminals.Open("t-1", shell))

		require.NoError(t, terminals.Close("t-1"))

		assert.Equal(t, mock.Event{Name: desktop.EventTerminalExit, Data: desktop.TerminalExit{ID: "t-1"}}, emitter.Next(t))
		assert.True(t, terminal.IsClosed())
	})

	t.Run("a terminal that cannot be opened is not", func(t *testing.T) {
		t.Parallel()

		terminals, opener, emitter, w := open(t, nil)
		failure := errors.New("the VM is not running")
		opener.On("Open", testify.Anything, w.Cli(), shell).Return(nil, failure).Once()

		assert.ErrorIs(t, terminals.Open("t-1", shell), failure)
		assert.Error(t, terminals.Write("t-1", nil))
		assert.True(t, emitter.Quiet())
	})

	t.Run("a terminal needs an id, of its own", func(t *testing.T) {
		t.Parallel()

		first, second := mock.NewTerminal(), mock.NewTerminal()
		terminals, opener, emitter, w := open(t, nil)
		opener.On("Open", testify.Anything, w.Cli(), shell).Return(first, nil).Once()
		opener.On("Open", testify.Anything, w.Cli(), shell).Return(second, nil).Once()

		var invalid desktop.InvalidError
		require.ErrorAs(t, terminals.Open("", shell), &invalid)
		assert.Equal(t, map[string]string{"id": "a terminal needs an id"}, invalid.Fields)

		require.NoError(t, terminals.Open("t-1", shell))
		assert.EqualError(t, terminals.Open("t-1", shell), `a terminal "t-1" is open already`)
		assert.True(t, second.IsClosed())
		assert.False(t, first.IsClosed())

		first.Say([]byte("still here"))
		assert.Equal(t, desktop.TerminalOutput{ID: "t-1", Data: []byte("still here")}, emitter.Next(t).Data)
	})

	t.Run("every terminal is closed as the app quits", func(t *testing.T) {
		t.Parallel()

		first, second := mock.NewTerminal(), mock.NewTerminal()
		second.CloseErr = errors.New("already gone")
		terminals, opener, _, w := open(t, nil)
		opener.On("Open", testify.Anything, w.Cli(), shell).Return(first, nil).Once()
		opener.On("Open", testify.Anything, w.Cli(), shell).Return(second, nil).Once()
		require.NoError(t, terminals.Open("t-1", shell))
		require.NoError(t, terminals.Open("t-2", shell))

		err := terminals.ServiceShutdown()

		assert.ErrorIs(t, err, second.CloseErr)
		assert.True(t, first.IsClosed())
		assert.True(t, second.IsClosed())
	})
}
