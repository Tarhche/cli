package remote

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/portforward"
	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestForwardVMCommand(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, ingress *streamtest.Ingress) *harnessed {
		t.Helper()

		base, r, terminal := harness(t, ingress, "")

		return &harnessed{Harness: base, remote: r, terminal: terminal, ingress: ingress}
	}

	actions := func(ingress *streamtest.Ingress) []string {
		var asked []string
		for _, request := range ingress.Requests() {
			asked = append(asked, request.Action)
		}

		return asked
	}

	t.Run("a port the vm publishes, through the forward action", func(t *testing.T) {
		t.Parallel()

		echo := echoServer(t)
		h := setup(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) { streamtest.Proxy(r, echo) }})

		f := forward(t, h, newForwardVMCommand(h.remote), "machine", "0:8080")
		port := f.port(t)

		assert.Equal(t, "ping", roundTrip(t, port, "ping"))
		assert.Eventually(t, func() bool { return strings.Contains(f.out.String(), "Handling connection for "+port+"\n") }, 5*time.Second, time.Millisecond)
		assert.Zero(t, f.stop(), f.errOut.String())

		requests := h.ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, streamtest.Request{Kind: "vms", UUID: machine.UUID, Action: "forward", Payload: `{"port":8080}`, Protocols: requests[0].Protocols, UserAgent: requests[0].UserAgent}, requests[0])
	})

	t.Run("a port a docker vm does not publish, relayed by nc inside it", func(t *testing.T) {
		t.Parallel()

		echo := echoServer(t)
		h := setup(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			if strings.Join(e.Payload.Command, " ") != "nc 127.0.0.1 9090" {
				_ = e.Fail("not nc")

				return
			}

			streamtest.Relay(e, echo)
		}})

		f := forward(t, h, newForwardVMCommand(h.remote), "dock", ":9090")
		port := f.port(t)

		assert.Equal(t, "ping", roundTrip(t, port, "ping"))
		assert.Zero(t, f.stop(), f.errOut.String())
		assert.Equal(t, []string{"exec"}, actions(h.ingress))
	})

	t.Run("a docker vm whose forward is refused is relayed to, from then on", func(t *testing.T) {
		t.Parallel()

		echo := echoServer(t)
		h := setup(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { streamtest.Relay(e, echo) }})

		f := forward(t, h, newForwardVMCommand(h.remote), "dock", "0:8080")
		port := f.port(t)

		assert.Equal(t, "one", roundTrip(t, port, "one"))
		assert.Equal(t, "two", roundTrip(t, port, "two"))
		assert.Zero(t, f.stop(), f.errOut.String())
		assert.Equal(t, []string{"forward", "exec", "exec"}, actions(h.ingress))
	})

	t.Run("a machine whose forward is refused says so for each connection", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{})

		f := forward(t, h, newForwardVMCommand(h.remote), "machine", "0:8080")
		port := f.port(t)

		assert.Empty(t, roundTrip(t, port, "ping"))
		assert.Eventually(t, func() bool {
			return strings.Contains(f.errOut.String(), "-> 8080: vm machine does not publish port 8080, or its ingress is denied: cannot open vms/0190-machine/forward: it is not there, or not yours to open (no such stream)\n")
		}, 5*time.Second, time.Millisecond, f.errOut.String())
		assert.Zero(t, f.stop())
	})

	t.Run("a refusal other than not being found is said as it is", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return 429, "" }})

		f := forward(t, h, newForwardVMCommand(h.remote), "dock", "0:8080")
		port := f.port(t)

		assert.Empty(t, roundTrip(t, port, "ping"))
		assert.Eventually(t, func() bool {
			return strings.Contains(f.errOut.String(), "-> 8080: cannot open vms/0190-docker/forward: too many streams were opened from here; try again shortly\n")
		}, 5*time.Second, time.Millisecond, f.errOut.String())
		assert.Zero(t, f.stop())
	})

	t.Run("anybody's vm with -A, a port it publishes through the forward action", func(t *testing.T) {
		t.Parallel()

		echo := echoServer(t)
		h := setup(t, &streamtest.Ingress{Forward: func(r *streamtest.Raw) { streamtest.Proxy(r, echo) }})

		f := forward(t, h, newForwardVMCommand(h.remote), "-A", "theirs", "0:8080")
		port := f.port(t)

		assert.Equal(t, "ping", roundTrip(t, port, "ping"))
		assert.Zero(t, f.stop(), f.errOut.String())

		requests := h.ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, theirs.UUID, requests[0].UUID)
		assert.Equal(t, "forward", requests[0].Action)
	})

	t.Run("a stream refused for the roles says, for each connection, the permission it needs: forward for a port the vm publishes, exec for one relayed", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			args       []string
			remote     string
			permission string
		}{
			{args: []string{"machine", "0:8080"}, remote: "8080", permission: "self.workload.vms.forward"},
			{args: []string{"-A", "machine", "0:8080"}, remote: "8080", permission: "workload.vms.forward"},
			{args: []string{"dock", "0:9090"}, remote: "9090", permission: "self.workload.vms.exec"},
			{args: []string{"-A", "theirs", "0:9090"}, remote: "9090", permission: "workload.vms.exec"},
		}

		for _, c := range cases {
			t.Run(strings.Join(c.args, " "), func(t *testing.T) {
				t.Parallel()

				h := setup(t, &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return http.StatusForbidden, "forbidden" }})

				f := forward(t, h, newForwardVMCommand(h.remote), c.args...)
				port := f.port(t)

				assert.Empty(t, roundTrip(t, port, "ping"))
				assert.Eventually(t, func() bool {
					return strings.Contains(f.errOut.String(), "-> "+c.remote+`: permission denied: this needs the "`+c.permission+`" permission, which none of your roles gives you`+"\n")
				}, 5*time.Second, time.Millisecond, f.errOut.String())
				assert.Zero(t, f.stop())
			})
		}
	})

	t.Run("localhost is both loopback addresses, on one port", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{})

		f := forward(t, h, newForwardVMCommand(h.remote), "--address", "localhost", "machine", "0:8080")
		port := f.port(t)

		if listener, err := net.Listen("tcp", "[::1]:0"); err == nil {
			_ = listener.Close()

			assert.Eventually(t, func() bool { return strings.Contains(f.out.String(), "Forwarding from [::1]:"+port+" -> 8080\n") }, 5*time.Second, time.Millisecond)
		}

		assert.Zero(t, f.stop())
	})

	t.Run("localhost is the IPv4 loopback alone where the IPv6 one cannot be listened on", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{})
		h.remote.forwarding = []portforward.Option{portforward.WithListener(func(ctx context.Context, network, address string) (net.Listener, error) {
			if strings.HasPrefix(address, "[::1]") {
				return nil, errors.New("no IPv6 here")
			}

			return (&net.ListenConfig{}).Listen(ctx, network, address)
		})}

		f := forward(t, h, newForwardVMCommand(h.remote), "--address", "localhost", "machine", "0:8080")
		port := f.port(t)

		assert.Zero(t, f.stop())
		assert.Equal(t, "Forwarding from 127.0.0.1:"+port+" -> 8080\n", f.out.String())
	})

	t.Run("a forward that cannot accept connections any more stops the command", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{})
		h.remote.forwarding = []portforward.Option{portforward.WithListener(func(context.Context, string, string) (net.Listener, error) {
			return brokenListener{}, nil
		})}

		assert.Equal(t, 1, h.Run(newForwardVMCommand(h.remote), "machine", "8080"))
		assert.Equal(t, "Forwarding from 127.0.0.1:1 -> 8080\n", h.Out.String())
		assert.Equal(t, "Error: forwarding 127.0.0.1:1 stopped: accepting failed\n", h.Err.String())
	})

	t.Run("a local port that is taken is the error", func(t *testing.T) {
		t.Parallel()

		taken, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		t.Cleanup(func() { _ = taken.Close() })

		h := setup(t, &streamtest.Ingress{})
		port := strings.TrimPrefix(taken.Addr().String(), "127.0.0.1:")

		assert.Equal(t, 1, h.Run(newForwardVMCommand(h.remote), "machine", port+":8080"))
		assert.Contains(t, h.Err.String(), "Error: listening on 127.0.0.1:"+port)
	})

	t.Run("what cannot be forwarded is the error", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			args []string
			err  string
		}{
			"a port a machine does not publish": {args: []string{"machine", "9090"}, err: "Error: vm machine does not publish port 9090, and runs no dockerd to relay it from inside: publish the port to forward it\n"},
			"a port that is not one":            {args: []string{"machine", "http"}, err: "Error: invalid port \"http\": the remote port has to be a number from 1 to 65535\n"},
			"an address that is not one":        {args: []string{"--address", "example.com", "machine", "8080"}, err: "Error: \"example.com\" is not an IP address or localhost\n"},
			"a vm that is not there":            {args: []string{"nope", "8080"}, err: "Error: no such vm: nope\n"},
		}

		for name, c := range cases {
			h := setup(t, &streamtest.Ingress{})

			assert.Equal(t, 1, h.Run(newForwardVMCommand(h.remote), c.args...), name)
			assert.Equal(t, c.err, h.Err.String(), name)
		}

		h := setup(t, &streamtest.Ingress{})
		withoutIngress(t, h.Harness)
		assert.Equal(t, 1, h.Run(newForwardVMCommand(h.remote), "machine", "8080"))
	})

	t.Run("a vm and a port are required", func(t *testing.T) {
		t.Parallel()

		h := setup(t, &streamtest.Ingress{})

		assert.Equal(t, 2, h.Run(newForwardVMCommand(h.remote), "machine"))
	})
}
