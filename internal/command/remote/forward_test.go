package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePorts(t *testing.T) {
	t.Parallel()

	t.Run("a remote port, a local port beside it, and any local port", func(t *testing.T) {
		t.Parallel()

		got, err := parsePorts([]string{"8080", "9000:80", ":443", "0:22"})

		require.NoError(t, err)
		assert.Equal(t, []portSpec{{local: 8080, remote: 8080}, {local: 9000, remote: 80}, {local: 0, remote: 443}, {local: 0, remote: 22}}, got)
	})

	t.Run("what is not a port is an error", func(t *testing.T) {
		t.Parallel()

		for _, arg := range []string{"http", "0", "70000", "8080:", "x:80", "70000:80", "-1:80"} {
			_, err := parsePorts([]string{arg})
			assert.Error(t, err, arg)
		}
	})
}

func TestParseAddresses(t *testing.T) {
	t.Parallel()

	got, err := parseAddresses("127.0.0.1, ::1,localhost")

	require.NoError(t, err)
	assert.Equal(t, []listenAddress{{ip: "127.0.0.1"}, {ip: "::1"}, {ip: "127.0.0.1"}, {ip: "::1", optional: true}}, got)

	_, err = parseAddresses("example.com")
	assert.EqualError(t, err, `"example.com" is not an IP address or localhost`)
}

func TestPortOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint16(8080), portOf(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}))
	assert.Zero(t, portOf(&net.UnixAddr{Name: "/run/socket", Net: "unix"}))
}

// echoServer is a TCP server on the loopback interface that sends back what
// it reads.
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

// forwarding is a port-forward command running in the background: what it
// writes, the local port it forwards from, and how it is stopped.
type forwarding struct {
	out    *syncBuffer
	errOut *syncBuffer
	status chan int
	cancel context.CancelFunc
}

var forwardingFrom = regexp.MustCompile(`Forwarding from 127\.0\.0\.1:(\d+) -> \d+`)

// forward runs cmd with args until it is stopped.
func forward(t *testing.T, h *harnessed, cmd console.Command, args ...string) *forwarding {
	t.Helper()

	f := &forwarding{out: &syncBuffer{}, errOut: &syncBuffer{}, status: make(chan int, 1)}
	h.Cli.Out, h.Cli.Err = f.out, f.errOut

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	go func() { f.status <- h.RunContext(ctx, cmd, args...) }()

	t.Cleanup(cancel)

	return f
}

// port is the local port the command forwards from, once it says it does.
func (f *forwarding) port(t *testing.T) string {
	t.Helper()

	var port string

	require.Eventually(t, func() bool {
		if found := forwardingFrom.FindStringSubmatch(f.out.String()); found != nil {
			port = found[1]

			return true
		}

		return false
	}, 10*time.Second, 5*time.Millisecond, "the command never forwarded: %s", f.errOut.String())

	return port
}

// stop interrupts the command, and is the status it exited with.
func (f *forwarding) stop() int {
	f.cancel()

	return <-f.status
}

// roundTrip sends what to the local port, and is what comes back.
func roundTrip(t *testing.T, port, what string) string {
	t.Helper()

	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	require.NoError(t, err)

	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte(what))
	require.NoError(t, err)

	answer := make([]byte, len(what))
	if _, err := io.ReadFull(conn, answer); err != nil {
		return ""
	}

	return string(answer)
}

// brokenListener is a listener that cannot accept connections.
type brokenListener struct{}

func (brokenListener) Accept() (net.Conn, error) { return nil, errors.New("accepting failed") }
func (brokenListener) Close() error              { return nil }
func (brokenListener) Addr() net.Addr            { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1} }
