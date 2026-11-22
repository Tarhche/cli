package remote

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestForwardContainerCommand(t *testing.T) {
	t.Parallel()

	inspect := []string{"docker", "inspect", "--format", containerAddresses, "abc123"}

	setup := func(t *testing.T, addresses string, inspectStatus int, relay string) *harnessed {
		t.Helper()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			switch {
			case slices.Equal(e.Payload.Command, inspect):
				if inspectStatus != 0 {
					_, _ = e.Stderr.Write([]byte("Error: No such object: abc123\n"))
				}

				_, _ = e.Stdout.Write([]byte(addresses))
				_ = e.Exit(inspectStatus)
			case slices.Equal(e.Payload.Command, []string{"nc", "172.17.0.5", "80"}):
				streamtest.Relay(e, relay)
			default:
				_ = e.Fail("unexpected command")
			}
		}}

		base, r, terminal := harness(t, ingress, "")

		return &harnessed{Harness: base, remote: r, terminal: terminal, ingress: ingress}
	}

	t.Run("a container's port, relayed by nc in its vm to its first address", func(t *testing.T) {
		t.Parallel()

		h := setup(t, "172.17.0.5 10.0.0.9 \n", 0, echoServer(t))

		f := forward(t, h, newForwardContainerCommand(h.remote), "api", "0:80")
		port := f.port(t)

		assert.Equal(t, "ping", roundTrip(t, port, "ping"))
		assert.Zero(t, f.stop(), f.errOut.String())

		for _, request := range h.ingress.Requests() {
			assert.Equal(t, dockerVM.UUID, request.UUID)
		}
	})

	t.Run("a container with no address cannot be forwarded to", func(t *testing.T) {
		t.Parallel()

		h := setup(t, " \n", 0, "")

		assert.Equal(t, 1, h.Run(newForwardContainerCommand(h.remote), "api", "80"))
		assert.Equal(t, "Error: container api has no address to forward to: is it running?\n", h.Err.String())
	})

	t.Run("a container that docker cannot inspect cannot be forwarded to", func(t *testing.T) {
		t.Parallel()

		h := setup(t, "", 1, "")

		assert.Equal(t, 1, h.Run(newForwardContainerCommand(h.remote), "api", "80"))
		assert.Equal(t, "Error: finding the address of container api: Error: No such object: abc123\n", h.Err.String())
	})

	t.Run("what cannot be forwarded is the error", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			args []string
			err  string
		}{
			"a port that is not one":     {args: []string{"api", "0"}, err: "Error: invalid port \"0\": the remote port has to be a number from 1 to 65535\n"},
			"an address that is not one": {args: []string{"--address", "nowhere", "api", "80"}, err: "Error: \"nowhere\" is not an IP address or localhost\n"},
			"a container not there":      {args: []string{"nope", "80"}, err: "Error: no such container: nope\n"},
		}

		for name, c := range cases {
			h := setup(t, "", 0, "")

			assert.Equal(t, 1, h.Run(newForwardContainerCommand(h.remote), c.args...), name)
			assert.Equal(t, c.err, h.Err.String(), name)
		}

		h := setup(t, "", 0, "")
		withoutIngress(t, h.Harness)
		assert.Equal(t, 1, h.Run(newForwardContainerCommand(h.remote), "api", "80"))
	})

	t.Run("a container and a port are required", func(t *testing.T) {
		t.Parallel()

		h := setup(t, "", 0, "")

		assert.Equal(t, 2, h.Run(newForwardContainerCommand(h.remote), "api"))
	})
}
