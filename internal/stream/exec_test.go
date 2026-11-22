package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

// syncBuffer is a buffer written and read from goroutines of their own.
type syncBuffer struct {
	lock   sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.buffer.String()
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("cannot write") }

func TestExec(t *testing.T) {
	t.Parallel()

	t.Run("a command is asked with what it runs, and a terminal's size only when it runs on one", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(0) }}
		d := serve(t, ingress)

		for _, options := range []ExecOptions{
			{Command: []string{"tar", "cf", "-", "/etc"}, Rows: 24, Cols: 80},
			{Command: []string{"sh"}, TTY: true, Rows: 24, Cols: 80, Env: []string{"A=b"}, WorkingDir: "/"},
		} {
			session, err := d.Exec(context.Background(), "0190", options)
			require.NoError(t, err)

			code, err := session.Wait()
			require.NoError(t, err)
			assert.Zero(t, code)
			require.NoError(t, session.Close())
		}

		requests := ingress.Requests()
		require.Len(t, requests, 2)
		assert.Equal(t, "vms", requests[0].Kind)
		assert.Equal(t, "0190", requests[0].UUID)
		assert.Equal(t, "exec", requests[0].Action)
		assert.JSONEq(t, `{"command":["tar","cf","-","/etc"]}`, requests[0].Payload)
		assert.JSONEq(t, `{"command":["sh"],"tty":true,"rows":24,"cols":80,"env":["A=b"],"working_dir":"/"}`, requests[1].Payload)
	})

	t.Run("a command is required, and its first word", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{}
		d := serve(t, ingress)

		_, err := d.Exec(context.Background(), "0190", ExecOptions{})
		assert.ErrorIs(t, err, ErrNoCommand)

		_, err = d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"", "x"}})
		assert.ErrorIs(t, err, ErrNoCommand)

		assert.Empty(t, ingress.Requests())
	})

	t.Run("a command killed by a signal exited with -1, which is no error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(-1) }})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"sleep", "9"}})
		require.NoError(t, err)

		code, err := session.Wait()

		require.NoError(t, err)
		assert.Equal(t, -1, code)
		require.NoError(t, session.Close())
	})

	t.Run("a refused command is the refusal", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{})

		_, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"true"}})

		assert.True(t, IsNotFound(err))
	})

	t.Run("its output is read, its errors written as they come, and its input ends when it is closed", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			input, err := io.ReadAll(e.Stdin)
			assert.NoError(t, err)
			assert.True(t, e.StdinEnded())

			_ = e.Raw(websocket.TextMessage, []byte("ignored"))
			_ = e.Raw(websocket.BinaryMessage, nil)
			_ = e.Send(9, []byte("a channel nobody knows"))
			_, _ = e.Stdout.Write([]byte("out:"))
			_, _ = e.Stderr.Write([]byte("warning\n"))
			_, _ = e.Stdout.Write(input)
			_ = e.Exit(3)
		}})

		stderr := &syncBuffer{}

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"cat"}, Stderr: stderr})
		require.NoError(t, err)

		t.Cleanup(func() { _ = session.Close() })

		_, err = session.Write([]byte("hello"))
		require.NoError(t, err)
		require.NoError(t, session.CloseWrite())
		require.NoError(t, session.CloseWrite())

		output, err := io.ReadAll(session)
		require.NoError(t, err)
		assert.Equal(t, "out:hello", string(output))
		assert.Equal(t, "warning\n", stderr.String())

		code, err := session.Wait()
		require.NoError(t, err)
		assert.Equal(t, 3, code)

		n, err := session.Read(nil)
		assert.Zero(t, n)
		assert.NoError(t, err)
	})

	t.Run("its errors are discarded when nobody takes them", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = e.Stderr.Write([]byte("warning\n"))
			_, _ = e.Stdout.Write([]byte("done"))
			_ = e.Exit(0)
		}})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"true"}})
		require.NoError(t, err)

		output, err := io.ReadAll(session)
		require.NoError(t, err)
		assert.Equal(t, "done", string(output))
		require.NoError(t, session.Close())
	})

	t.Run("a resize says the terminal's size, and a size of nothing says nothing", func(t *testing.T) {
		t.Parallel()

		sized := make(chan streamtest.Size, 1)

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			sized <- <-e.Resizes
			_ = e.Exit(0)
		}})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"sh"}, TTY: true})
		require.NoError(t, err)

		t.Cleanup(func() { _ = session.Close() })

		require.NoError(t, session.Resize(40, 0))
		require.NoError(t, session.Resize(40, 120))

		assert.Equal(t, streamtest.Size{Rows: 40, Cols: 120}, <-sized)

		_, err = session.Wait()
		require.NoError(t, err)
	})

	t.Run("a command that could not be waited for is an error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Fail("the vm went away") }})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"sleep", "9"}})
		require.NoError(t, err)

		code, err := session.Wait()

		assert.Equal(t, -1, code)
		assert.EqualError(t, err, "the command could not be waited for: the vm went away")
		require.NoError(t, session.Close())
	})

	t.Run("a stream that ends before the command's exit is an error", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			end  func(*streamtest.Exec)
			want string
		}{
			"closed":      {end: func(e *streamtest.Exec) { _ = e.CloseWith(websocket.CloseNormalClosure, "") }, want: "the stream ended before the command's exit was said"},
			"dropped":     {end: func(e *streamtest.Exec) { _ = e.Drop() }, want: "the stream ended before the command's exit was said"},
			"failed":      {end: func(e *streamtest.Exec) { _ = e.CloseWith(websocket.CloseInternalServerErr, "no such vm") }, want: "the stream was closed: no such vm (1011)"},
			"bad exit":    {end: func(e *streamtest.Exec) { _ = e.Send(3, []byte("{")) }, want: "reading how the command exited: unexpected EOF"},
			"half stdout": {end: func(e *streamtest.Exec) { _ = e.Send(1, []byte("partial")); _ = e.Drop() }, want: "the stream ended before the command's exit was said"},
			"cut stdout":  {end: func(e *streamtest.Exec) { _ = e.Frame([]byte{0x02, 0x02, 1, 'a'}); _ = e.Drop() }, want: "the stream ended before the command's exit was said"},
			"cut channel": {end: func(e *streamtest.Exec) { _ = e.Frame([]byte{0x02, 0x00}); _ = e.Drop() }, want: "the stream ended before the command's exit was said"},
		}

		for name, c := range cases {
			d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { c.end(e) }})

			session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"true"}})
			require.NoError(t, err, name)

			code, err := session.Wait()

			assert.Equal(t, -1, code, name)
			assert.EqualError(t, err, c.want, name)

			_, err = session.Read(make([]byte, 1))
			assert.EqualError(t, err, c.want, name)

			require.NoError(t, session.Close(), name)
		}
	})

	t.Run("errors that cannot be written stop the output", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = e.Stderr.Write([]byte("warning"))
			_ = e.Exit(0)
		}})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"true"}, Stderr: failingWriter{}})
		require.NoError(t, err)

		_, err = io.ReadAll(session)

		assert.EqualError(t, err, "cannot write")
		require.NoError(t, session.Close())
	})

	t.Run("a command closed here is let go of", func(t *testing.T) {
		t.Parallel()

		gone := make(chan struct{})

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			<-e.Done
			close(gone)
		}})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"sleep", "9"}})
		require.NoError(t, err)

		require.NoError(t, session.Close())
		<-gone

		_, err = session.Read(make([]byte, 1))
		assert.ErrorIs(t, err, net.ErrClosed)

		_, err = session.Write([]byte("x"))
		assert.ErrorIs(t, err, net.ErrClosed)
		assert.ErrorIs(t, session.CloseWrite(), net.ErrClosed)
	})
}

func TestRunner(t *testing.T) {
	t.Parallel()

	echo := func(e *streamtest.Exec) {
		input, err := io.ReadAll(e.Stdin)
		if err != nil {
			_ = e.Fail(err.Error())

			return
		}

		_, _ = e.Stdout.Write(bytes.ToUpper(input))
		_ = e.Exit(0)
	}

	t.Run("a command is run to its end, its input given and its output taken", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: echo}
		d := serve(t, ingress)

		var stdout bytes.Buffer

		err := d.Runner("0190").Run(context.Background(), []string{"tr", "a-z", "A-Z"}, strings.NewReader("hello"), &stdout)

		require.NoError(t, err)
		assert.Equal(t, "HELLO", stdout.String())

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, "0190", requests[0].UUID)
		assert.JSONEq(t, `{"command":["tr","a-z","A-Z"]}`, requests[0].Payload)
	})

	t.Run("a command given no input reads its end", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: echo})

		var stdout bytes.Buffer

		require.NoError(t, d.Runner("0190").Run(context.Background(), []string{"cat"}, nil, &stdout))
		assert.Empty(t, stdout.String())
	})

	t.Run("a command that fails is an exit error saying what it said", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = e.Stderr.Write([]byte("tar: /x: Cannot stat\n"))
			_, _ = e.Stderr.Write(bytes.Repeat([]byte("y"), maxStderr))
			_ = e.Exit(2)
		}})

		err := d.Runner("0190").Run(context.Background(), []string{"tar", "cf", "-", "/x"}, nil, io.Discard)

		var exitErr *ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 2, exitErr.Code)
		assert.Len(t, exitErr.Stderr, maxStderr)
		assert.True(t, strings.HasPrefix(exitErr.Stderr, "tar: /x: Cannot stat\n"))
	})

	t.Run("a command that could not be waited for is the error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Fail("lost") }})

		err := d.Runner("0190").Run(context.Background(), []string{"true"}, nil, io.Discard)

		assert.EqualError(t, err, "the command could not be waited for: lost")
	})

	t.Run("a command that cannot be run is the error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{})

		err := d.Runner("0190").Run(context.Background(), []string{"true"}, nil, io.Discard)

		assert.True(t, IsNotFound(err))
	})

	t.Run("input that cannot be read stops the command, and is the error", func(t *testing.T) {
		t.Parallel()

		gone := make(chan struct{})

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			<-e.Done
			close(gone)
		}})

		err := d.Runner("0190").Run(context.Background(), []string{"cat"}, failingReader{}, io.Discard)

		assert.EqualError(t, err, "reading the command's input: broken")
		<-gone
	})

	t.Run("input the command does not wait for is let go of", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_ = e.Exit(0)
		}})

		reader, writer := io.Pipe()
		t.Cleanup(func() { _ = writer.Close() })

		go func() {
			for {
				if _, err := writer.Write([]byte("more")); err != nil {
					return
				}
			}
		}()

		require.NoError(t, d.Runner("0190").Run(context.Background(), []string{"true"}, reader, io.Discard))
	})

	t.Run("output that cannot be written stops the command, and is the error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = e.Stdout.Write([]byte("data"))
			<-e.Done
		}})

		err := d.Runner("0190").Run(context.Background(), []string{"cat"}, nil, failingWriter{})

		assert.EqualError(t, err, "cannot write")
	})

	t.Run("a command stopped by its context is the context's error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			cancel()
			<-e.Done
		}})

		err := d.Runner("0190").Run(ctx, []string{"sleep", "9"}, nil, io.Discard)

		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestRelay(t *testing.T) {
	t.Parallel()

	t.Run("nc relays a connection: what is written is sent, its writing half closes, and the answer is read to its end", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			input, err := io.ReadAll(e.Stdin)
			assert.NoError(t, err)

			_, _ = e.Stdout.Write(append([]byte("echo:"), input...))
			_ = e.Exit(0)
		}}
		d := serve(t, ingress)

		conn, err := d.Relay(context.Background(), "0190", "172.17.0.2", 80)
		require.NoError(t, err)

		t.Cleanup(func() { _ = conn.Close() })

		_, err = conn.Write([]byte("ping"))
		require.NoError(t, err)
		require.NoError(t, conn.CloseWrite())

		answer, err := io.ReadAll(conn)
		require.NoError(t, err)
		assert.Equal(t, "echo:ping", string(answer))

		requests := ingress.Requests()
		require.Len(t, requests, 1)

		var payload struct {
			Command []string `json:"command"`
		}

		require.NoError(t, json.Unmarshal([]byte(requests[0].Payload), &payload))
		assert.Equal(t, []string{"nc", "172.17.0.2", "80"}, payload.Command)
	})

	t.Run("nc that fails is an exit error saying why", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = e.Stderr.Write([]byte("nc: can't connect to remote host (127.0.0.1): Connection refused\n"))
			_ = e.Exit(1)
		}})

		conn, err := d.Relay(context.Background(), "0190", "127.0.0.1", 81)
		require.NoError(t, err)

		_, err = io.ReadAll(conn)

		var exitErr *ExitError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 1, exitErr.Code)
		assert.EqualError(t, err, "nc could not relay to 127.0.0.1:81 in the vm: nc: can't connect to remote host (127.0.0.1): Connection refused")
		require.NoError(t, conn.Close())
	})

	t.Run("nc that fails saying nothing says where it relayed to", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(1) }})

		conn, err := d.Relay(context.Background(), "0190", "::1", 8080)
		require.NoError(t, err)

		_, err = io.ReadAll(conn)

		assert.EqualError(t, err, "nc could not relay to [::1]:8080 in the vm: exit status 1")
		require.NoError(t, conn.Close())
	})

	t.Run("nc that could not be waited for is the error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Fail("lost") }})

		conn, err := d.Relay(context.Background(), "0190", "127.0.0.1", 81)
		require.NoError(t, err)

		_, err = io.ReadAll(conn)

		assert.EqualError(t, err, "the command could not be waited for: lost")
		require.NoError(t, conn.Close())
	})

	t.Run("a relay that cannot be opened is the error", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{})

		_, err := d.Relay(context.Background(), "0190", "127.0.0.1", 81)

		assert.True(t, IsNotFound(err))
	})
}

func TestFeed(t *testing.T) {
	t.Parallel()

	t.Run("input is no longer read once the command is gone", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { <-e.Done }})

		session, err := d.Exec(context.Background(), "0190", ExecOptions{Command: []string{"cat"}})
		require.NoError(t, err)
		require.NoError(t, session.Close())

		unread := make(chan error, 1)
		input := strings.NewReader("more")

		feed(session, input, unread)

		assert.Empty(t, unread)
		assert.Zero(t, input.Len())
	})
}

func TestFirstBytes(t *testing.T) {
	t.Parallel()

	kept := &firstBytes{limit: 5}

	n, err := kept.Write([]byte("abc"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	n, err = kept.Write([]byte("defgh"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = kept.Write([]byte("ij"))
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	assert.Equal(t, "abcde", kept.String())
}
