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

// echoServer is a TCP server on the loopback interface that sends back what
// it reads, until its client closes its writing half.
func echoServer(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer func() { _ = conn.Close() }()

				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	return listener.Addr().String()
}

func TestForward(t *testing.T) {
	t.Parallel()

	t.Run("a forward is asked for its port, and carries the connection's bytes both ways", func(t *testing.T) {
		t.Parallel()

		address := echoServer(t)
		ingress := &streamtest.Ingress{Forward: func(r *streamtest.Raw) { streamtest.Proxy(r, address) }}
		d := serve(t, ingress, WithUserAgent("tarhche-cli/test"))

		conn, err := d.Forward(context.Background(), "0190", 8080)
		require.NoError(t, err)

		t.Cleanup(func() { _ = conn.Close() })

		long := bytes.Repeat([]byte("abc"), maxMessage)

		_, err = conn.Write(long)
		require.NoError(t, err)

		echoed := make([]byte, len(long))
		_, err = io.ReadFull(conn, echoed)
		require.NoError(t, err)
		assert.Equal(t, long, echoed)

		assert.NotNil(t, conn.LocalAddr())
		assert.NotNil(t, conn.RemoteAddr())

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, streamtest.Request{Kind: "vms", UUID: "0190", Action: "forward", Payload: `{"port":8080}`, Protocols: []string{"bearer", streamtest.Token}, UserAgent: "tarhche-cli/test"}, requests[0])
	})

	t.Run("the connection's end is read as its end, its loss as an unexpected one, and a port that could not be dialled as why", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			end  func(*streamtest.Raw)
			want error
		}{
			"closed":     {end: func(r *streamtest.Raw) { _ = r.Close() }, want: io.EOF},
			"going away": {end: func(r *streamtest.Raw) { _ = r.CloseWith(websocket.CloseGoingAway, "") }, want: io.EOF},
			"no status":  {end: func(r *streamtest.Raw) { _ = r.CloseWith(websocket.CloseNoStatusReceived, "") }, want: io.EOF},
			"lost":       {end: func(r *streamtest.Raw) { _ = r.Drop() }, want: io.ErrUnexpectedEOF},
			"refused":    {end: func(r *streamtest.Raw) { _ = r.Fail("dial tcp 10.0.0.2:8080: connect: connection refused") }, want: &CloseError{Code: websocket.CloseInternalServerErr, Reason: "dial tcp 10.0.0.2:8080: connect: connection refused"}},
		}

		for name, c := range cases {
			d := serve(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) {
				_, _ = r.Write([]byte("hi"))
				c.end(r)
			}})

			conn, err := d.Forward(context.Background(), "0190", 8080)
			require.NoError(t, err, name)

			read, err := io.ReadAll(conn)
			assert.Equal(t, "hi", string(read), name)

			if errors.Is(c.want, io.EOF) {
				assert.NoError(t, err, name)
			} else {
				assert.Equal(t, c.want, err, name)
			}

			require.NoError(t, conn.Close(), name)
		}
	})

	t.Run("a deadline that passed stops reading", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) { _, _ = io.Copy(io.Discard, r) }})

		conn, err := d.Forward(context.Background(), "0190", 8080)
		require.NoError(t, err)

		t.Cleanup(func() { _ = conn.Close() })

		require.NoError(t, conn.SetDeadline(time.Now().Add(-time.Second)))

		var timeout net.Error

		_, err = conn.Read(make([]byte, 1))
		require.ErrorAs(t, err, &timeout)
		assert.True(t, timeout.Timeout())

		_, err = conn.Write([]byte("x"))
		require.ErrorAs(t, err, &timeout)
		assert.True(t, timeout.Timeout())
	})

	t.Run("a connection closed here reads that it was closed, and takes no deadline", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) { _, _ = io.Copy(io.Discard, r) }})

		conn, err := d.Forward(context.Background(), "0190", 8080)
		require.NoError(t, err)

		require.NoError(t, conn.Close())

		_, err = conn.Read(make([]byte, 1))
		assert.ErrorIs(t, err, net.ErrClosed)
		assert.Error(t, conn.SetDeadline(time.Now()))
	})

	t.Run("a message cut short is read as far as it went", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) {
			_ = r.Frame([]byte{0x02, 0x02, 'h', 'i'})
			_ = r.Drop()
		}})

		conn, err := d.Forward(context.Background(), "0190", 8080)
		require.NoError(t, err)

		read, err := io.ReadAll(conn)

		assert.Equal(t, "hi", string(read))
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		require.NoError(t, conn.Close())
	})

	t.Run("a refused forward is the refusal", func(t *testing.T) {
		t.Parallel()

		d := serve(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) {}, Refuse: func(streamtest.Request) (int, string) {
			return 400, `{"errors":{"port":"is not one the vm publishes"}}`
		}})

		_, err := d.Forward(context.Background(), "0190", 8081)

		var refused *Error
		require.ErrorAs(t, err, &refused)
		assert.Equal(t, map[string]string{"port": "is not one the vm publishes"}, refused.Fields)
	})
}
