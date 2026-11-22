package portforward

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder keeps the events a forwarder tells.
type recorder struct {
	lock   sync.Mutex
	events []Event
}

func (r *recorder) record(event Event) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.events = append(r.events, event)
}

func (r *recorder) kinds() []EventKind {
	r.lock.Lock()
	defer r.lock.Unlock()

	kinds := make([]EventKind, 0, len(r.events))
	for _, event := range r.events {
		kinds = append(kinds, event.Kind)
	}

	return kinds
}

func (r *recorder) failures() []error {
	r.lock.Lock()
	defer r.lock.Unlock()

	var failures []error

	for _, event := range r.events {
		if event.Kind == Failed {
			failures = append(failures, event.Err)
		}
	}

	return failures
}

// answering is a TCP server on the loopback interface that reads what its
// client sends until the client closes its writing half, then answers
// "got:" and what it read, and closes the connection.
func answering(t *testing.T) string {
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

				read, _ := io.ReadAll(conn)
				_, _ = conn.Write(append([]byte("got:"), read...))
			}()
		}
	}()

	return listener.Addr().String()
}

// dialTCP dials address for every connection, as it is: a TCP connection,
// whose writing half closes on its own.
func dialTCP(address string) Dial {
	return func(ctx context.Context) (io.ReadWriteCloser, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}
}

// whole is a connection whose writing half does not close on its own.
type whole struct {
	io.ReadWriteCloser
}

// remote is a connection to the other side that fails as a test says.
type remote struct {
	readErr       error
	writeErr      error
	closeWriteErr error
	closed        chan struct{}
	once          sync.Once
}

func newRemote() *remote {
	return &remote{closed: make(chan struct{})}
}

func (r *remote) Read([]byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}

	<-r.closed

	return 0, net.ErrClosed
}

func (r *remote) Write(p []byte) (int, error) {
	if r.writeErr != nil {
		return 0, r.writeErr
	}

	return len(p), nil
}

func (r *remote) CloseWrite() error {
	return r.closeWriteErr
}

func (r *remote) Close() error {
	r.once.Do(func() { close(r.closed) })

	return nil
}

func start(t *testing.T, f *Forwarder, spec Spec) *Forward {
	t.Helper()

	fw, err := f.Start(spec)
	require.NoError(t, err)

	t.Cleanup(func() { _ = fw.Close() })

	return fw
}

func connect(t *testing.T, fw *Forward) *net.TCPConn {
	t.Helper()

	conn, err := net.Dial("tcp", fw.Addr().String())
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return conn.(*net.TCPConn)
}

func TestEventKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "accepted", Accepted.String())
	assert.Equal(t, "failed", Failed.String())
	assert.Equal(t, "closed", Closed.String())
	assert.Equal(t, "EventKind(9)", EventKind(9).String())
}

func TestForwarderStart(t *testing.T) {
	t.Parallel()

	t.Run("a forward listens on the loopback interface, on any free port, unless told otherwise", func(t *testing.T) {
		t.Parallel()

		f := New()

		first := start(t, f, Spec{Remote: "8080", Dial: dialTCP("127.0.0.1:1")})
		second := start(t, f, Spec{Address: "127.0.0.1", Port: 0, Remote: "9090", Dial: dialTCP("127.0.0.1:1")})

		address, ok := first.Addr().(*net.TCPAddr)
		require.True(t, ok)
		assert.Equal(t, "127.0.0.1", address.IP.String())
		assert.NotZero(t, address.Port)

		assert.Equal(t, 1, first.ID)
		assert.Equal(t, 2, second.ID)
		assert.Equal(t, "8080", first.Remote())
		assert.Equal(t, []*Forward{first, second}, f.Forwards())

		found, ok := f.Forward(2)
		assert.True(t, ok)
		assert.Same(t, second, found)

		_, ok = f.Forward(3)
		assert.False(t, ok)
	})

	t.Run("a forward needs a dial", func(t *testing.T) {
		t.Parallel()

		_, err := New().Start(Spec{})

		assert.Error(t, err)
	})

	t.Run("an address that cannot be listened on is an error", func(t *testing.T) {
		t.Parallel()

		f := New()
		taken := start(t, f, Spec{Dial: dialTCP("127.0.0.1:1")})

		_, err := f.Start(Spec{Port: uint16(taken.Addr().(*net.TCPAddr).Port), Dial: dialTCP("127.0.0.1:1")})

		assert.Error(t, err)
		assert.Len(t, f.Forwards(), 1)
	})
}

func TestForward(t *testing.T) {
	t.Parallel()

	t.Run("a connection is piped to the other side, which reads its end and answers", func(t *testing.T) {
		t.Parallel()

		events := &recorder{}
		fw := start(t, New(WithEvents(events.record)), Spec{Dial: dialTCP(answering(t))})

		conn := connect(t, fw)

		_, err := conn.Write([]byte("ping"))
		require.NoError(t, err)
		require.NoError(t, conn.CloseWrite())

		answer, err := io.ReadAll(conn)
		require.NoError(t, err)
		assert.Equal(t, "got:ping", string(answer))

		assert.Eventually(t, func() bool { return len(events.kinds()) == 2 }, 5*time.Second, time.Millisecond)
		assert.Equal(t, []EventKind{Accepted, Closed}, events.kinds())
		assert.Equal(t, conn.LocalAddr().String(), events.events[0].Client.String())
		assert.Same(t, fw, events.events[0].Forward)
	})

	t.Run("a connection whose other side cannot close its writing half closes it all at its end", func(t *testing.T) {
		t.Parallel()

		address := answering(t)
		fw := start(t, New(), Spec{Dial: func(ctx context.Context) (io.ReadWriteCloser, error) {
			conn, err := dialTCP(address)(ctx)
			if err != nil {
				return nil, err
			}

			return whole{conn}, nil
		}})

		conn := connect(t, fw)

		_, err := conn.Write([]byte("ping"))
		require.NoError(t, err)
		require.NoError(t, conn.CloseWrite())

		answer, err := io.ReadAll(conn)
		require.NoError(t, err)
		assert.Empty(t, answer)
	})

	t.Run("several connections are served at once, and counted", func(t *testing.T) {
		t.Parallel()

		fw := start(t, New(), Spec{Dial: dialTCP(answering(t))})

		first, second := connect(t, fw), connect(t, fw)

		assert.Eventually(t, func() bool { return fw.Connections() == 2 }, 5*time.Second, time.Millisecond)

		for _, conn := range []*net.TCPConn{second, first} {
			_, err := conn.Write([]byte("x"))
			require.NoError(t, err)
			require.NoError(t, conn.CloseWrite())

			answer, err := io.ReadAll(conn)
			require.NoError(t, err)
			assert.Equal(t, "got:x", string(answer))
		}

		assert.Eventually(t, func() bool { return fw.Connections() == 0 }, 5*time.Second, time.Millisecond)
	})

	t.Run("what stops the other side short is told, and the connection closed", func(t *testing.T) {
		t.Parallel()

		dialErr := errors.New("cannot dial")
		readErr := errors.New("nc: connection refused")
		writeErr := errors.New("cannot write")
		closeWriteErr := errors.New("cannot close the writing half")

		cases := map[string]struct {
			dial Dial
			use  func(*net.TCPConn)
			want error
		}{
			"dialling": {
				dial: func(context.Context) (io.ReadWriteCloser, error) { return nil, dialErr },
				want: dialErr,
			},
			"reading": {
				dial: func(context.Context) (io.ReadWriteCloser, error) {
					return &remote{readErr: readErr, closed: make(chan struct{})}, nil
				},
				want: readErr,
			},
			"writing": {
				dial: func(context.Context) (io.ReadWriteCloser, error) {
					return &remote{writeErr: writeErr, closed: make(chan struct{})}, nil
				},
				use:  func(conn *net.TCPConn) { _, _ = conn.Write([]byte("x")) },
				want: writeErr,
			},
			"closing the writing half": {
				dial: func(context.Context) (io.ReadWriteCloser, error) {
					return &remote{closeWriteErr: closeWriteErr, closed: make(chan struct{})}, nil
				},
				use:  func(conn *net.TCPConn) { _ = conn.CloseWrite() },
				want: closeWriteErr,
			},
		}

		for name, c := range cases {
			events := &recorder{}
			fw := start(t, New(WithEvents(events.record)), Spec{Dial: c.dial})

			conn := connect(t, fw)

			if c.use != nil {
				c.use(conn)
			}

			_, err := io.ReadAll(conn)
			assert.NoError(t, err, name)

			assert.Eventually(t, func() bool { return len(events.kinds()) == 3 }, 5*time.Second, time.Millisecond, name)
			assert.Equal(t, []EventKind{Accepted, Failed, Closed}, events.kinds(), name)
			assert.Equal(t, []error{c.want}, events.failures(), name)
		}
	})

	t.Run("a client that goes away is no failure", func(t *testing.T) {
		t.Parallel()

		events := &recorder{}
		other := newRemote()
		fw := start(t, New(WithEvents(events.record)), Spec{Dial: func(context.Context) (io.ReadWriteCloser, error) { return other, nil }})

		conn := connect(t, fw)
		require.NoError(t, conn.SetLinger(0))
		require.NoError(t, conn.Close())

		assert.Eventually(t, func() bool { return len(events.kinds()) == 2 }, 5*time.Second, time.Millisecond)
		assert.Equal(t, []EventKind{Accepted, Closed}, events.kinds())
		<-other.closed
	})

	t.Run("closing a forward ends its connections and its dials, and forgets it", func(t *testing.T) {
		t.Parallel()

		events := &recorder{}
		f := New(WithEvents(events.record))
		dialling := make(chan struct{})

		piped := start(t, f, Spec{Dial: func(context.Context) (io.ReadWriteCloser, error) { return newRemote(), nil }})
		dialled := start(t, f, Spec{Dial: func(ctx context.Context) (io.ReadWriteCloser, error) {
			close(dialling)
			<-ctx.Done()

			return nil, ctx.Err()
		}})

		pipedConn, dialledConn := connect(t, piped), connect(t, dialled)

		assert.Eventually(t, func() bool { return piped.Connections() == 1 }, 5*time.Second, time.Millisecond)
		<-dialling

		require.NoError(t, f.Close())
		require.NoError(t, piped.Close())

		for _, fw := range []*Forward{piped, dialled} {
			select {
			case <-fw.Done():
			default:
				t.Fatal("a closed forward is not done")
			}

			assert.NoError(t, fw.Err())
			assert.Zero(t, fw.Connections())
		}

		for _, conn := range []*net.TCPConn{pipedConn, dialledConn} {
			_, err := io.ReadAll(conn)
			assert.NoError(t, err)
		}

		assert.Empty(t, f.Forwards())
		assert.Empty(t, events.failures())

		_, err := net.Dial("tcp", piped.Addr().String())
		assert.Error(t, err)
	})

	t.Run("a forward that cannot accept connections any more stops, and says why", func(t *testing.T) {
		t.Parallel()

		acceptErr := errors.New("too many open files")
		listener := &brokenListener{accepted: make(chan struct{}), err: acceptErr}

		f := New()
		f.listen = func(context.Context, string, string) (net.Listener, error) { return listener, nil }

		fw, err := f.Start(Spec{Dial: dialTCP("127.0.0.1:1")})
		require.NoError(t, err)

		<-fw.Done()

		assert.Equal(t, acceptErr, fw.Err())
		assert.Empty(t, f.Forwards())
		assert.Equal(t, "fake:0", fw.Addr().String())
		assert.Equal(t, listener.closeErr, fw.Close())
	})

	t.Run("a listener that cannot be closed is the error", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("cannot close")
		listener := &brokenListener{accepted: make(chan struct{}), closeErr: closeErr}

		f := New()
		f.listen = func(context.Context, string, string) (net.Listener, error) { return listener, nil }

		fw, err := f.Start(Spec{Dial: dialTCP("127.0.0.1:1")})
		require.NoError(t, err)

		assert.ErrorIs(t, f.Close(), closeErr)
		assert.Equal(t, closeErr, fw.Close())
	})
}

// brokenListener is a listener whose Accept fails with err, or waits until
// it is closed when it has none.
type brokenListener struct {
	accepted chan struct{}
	err      error
	closeErr error
	once     sync.Once
}

func (l *brokenListener) Accept() (net.Conn, error) {
	if l.err != nil {
		return nil, l.err
	}

	<-l.accepted

	return nil, net.ErrClosed
}

func (l *brokenListener) Close() error {
	l.once.Do(func() { close(l.accepted) })

	return l.closeErr
}

func (l *brokenListener) Addr() net.Addr {
	return fakeAddr{}
}

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake:0" }
