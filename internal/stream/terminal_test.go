package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestTerminal(t *testing.T) {
	t.Parallel()

	t.Run("what is typed is the terminal's input, what it shows its output, and a resize is a control message", func(t *testing.T) {
		t.Parallel()

		said := make(chan []streamtest.Message, 1)

		d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			var messages []streamtest.Message

			for len(messages) < 2 {
				message, err := terminal.Next()
				if err != nil {
					break
				}

				messages = append(messages, message)
			}

			said <- messages

			_ = terminal.Text("ignored")
			_ = terminal.Output([]byte("total 0\r\n"))
			_ = terminal.End()
		}})

		terminal, err := d.Attach(context.Background(), KindTasks, "0190")
		require.NoError(t, err)

		t.Cleanup(func() { _ = terminal.Close() })

		require.NoError(t, terminal.Resize(0, 80))
		require.NoError(t, terminal.Resize(40, 120))

		n, err := terminal.Write([]byte("ls\n"))
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		output, err := io.ReadAll(terminal)
		require.NoError(t, err)
		assert.Equal(t, "total 0\r\n", string(output))

		assert.Equal(t, []streamtest.Message{
			{Control: &streamtest.Control{Type: "resize", Rows: 40, Cols: 120}},
			{Input: []byte("ls\n")},
		}, <-said)
	})

	t.Run("a long input is written as several messages", func(t *testing.T) {
		t.Parallel()

		typed := make(chan []byte, 1)
		long := bytes.Repeat([]byte("x"), 2*maxMessage+5)

		d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			var all []byte

			for range 3 {
				message, err := terminal.Next()
				if err != nil {
					break
				}

				assert.LessOrEqual(t, len(message.Input), maxMessage)
				all = append(all, message.Input...)
			}

			typed <- all

			_ = terminal.End()
		}})

		terminal, err := d.Attach(context.Background(), KindVMs, "0190")
		require.NoError(t, err)

		t.Cleanup(func() { _ = terminal.Close() })

		n, err := terminal.Write(long)
		require.NoError(t, err)
		assert.Equal(t, len(long), n)

		n, err = terminal.Write(nil)
		require.NoError(t, err)
		assert.Zero(t, n)

		assert.Equal(t, long, <-typed)
	})

	t.Run("its end however the shell let go of it, and an error when it was closed for another reason", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			end  func(*streamtest.Terminal)
			want error
		}{
			"closed":                {end: func(t *streamtest.Terminal) { _ = t.End() }, want: io.EOF},
			"going away":            {end: func(t *streamtest.Terminal) { _ = t.CloseWith(websocket.CloseGoingAway, "") }, want: io.EOF},
			"dropped":               {end: func(t *streamtest.Terminal) { _ = t.Drop() }, want: io.EOF},
			"closed with no status": {end: func(t *streamtest.Terminal) { _ = t.CloseWith(websocket.CloseNoStatusReceived, "") }, want: io.EOF},
			"failed":                {end: func(t *streamtest.Terminal) { _ = t.CloseWith(websocket.CloseInternalServerErr, "the shell crashed") }, want: &CloseError{Code: websocket.CloseInternalServerErr, Reason: "the shell crashed"}},
		}

		for name, c := range cases {
			d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
				_ = terminal.Output([]byte("bye"))
				c.end(terminal)
			}})

			terminal, err := d.Attach(context.Background(), KindVMs, "0190")
			require.NoError(t, err, name)

			output, err := io.ReadAll(terminal)
			assert.Equal(t, "bye", string(output), name)

			if errors.Is(c.want, io.EOF) {
				assert.NoError(t, err, name)
			} else {
				assert.Equal(t, c.want, err, name)
			}

			require.NoError(t, terminal.Close(), name)
		}
	})

	t.Run("a terminal closed here is let go of and reads its end", func(t *testing.T) {
		t.Parallel()

		gone := make(chan error, 1)

		d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			_, err := terminal.Next()
			gone <- err
		}})

		terminal, err := d.Attach(context.Background(), KindVMs, "0190")
		require.NoError(t, err)

		require.NoError(t, terminal.Close())
		require.NoError(t, terminal.Close())

		var closeErr *websocket.CloseError
		require.ErrorAs(t, <-gone, &closeErr)
		assert.Equal(t, websocket.CloseNormalClosure, closeErr.Code)

		n, err := terminal.Read(make([]byte, 8))
		assert.Zero(t, n)
		assert.ErrorIs(t, err, io.EOF)

		_, err = terminal.Write([]byte("x"))
		assert.ErrorIs(t, err, net.ErrClosed)

		n, err = terminal.Read(nil)
		assert.Zero(t, n)
		assert.NoError(t, err)
	})

	t.Run("an open stream is pinged", func(t *testing.T) {
		t.Parallel()

		pinged := make(chan struct{})

		d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			go func() {
				assert.Eventually(t, func() bool { return terminal.Pings() >= 2 }, 5*time.Second, 5*time.Millisecond)
				close(pinged)
			}()

			_, _ = terminal.Next()
		}}, WithPingInterval(10*time.Millisecond))

		terminal, err := d.Attach(context.Background(), KindVMs, "0190")
		require.NoError(t, err)

		<-pinged
		require.NoError(t, terminal.Close())
	})

	t.Run("a ping from the ingress is answered as the terminal is read", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			_ = terminal.Ping()
			_ = terminal.Output([]byte("ok"))
			_ = terminal.End()
		}})

		terminal, err := d.Attach(context.Background(), KindVMs, "0190")
		require.NoError(t, err)

		output, err := io.ReadAll(terminal)
		require.NoError(t, err)
		assert.Equal(t, "ok", string(output))
		require.NoError(t, terminal.Close())
	})
}

func TestTerminalEnded(t *testing.T) {
	t.Parallel()

	assert.Equal(t, io.ErrClosedPipe, terminalEnded(io.ErrClosedPipe))
	assert.Equal(t, io.EOF, terminalEnded(net.ErrClosed))
}
