package remote

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestExecContainerCommand(t *testing.T) {
	t.Parallel()

	t.Run("docker exec in the container's vm, with docker exec's flags", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			input, _ := io.ReadAll(e.Stdin)
			_, _ = e.Stdout.Write(input)
			_ = e.Exit(0)
		}}

		h, r, terminal := harness(t, ingress, "typed")
		terminal.terminals[h.Cli.In] = true

		status := h.Run(newExecContainerCommand(r), "-it", "-e", "A=b", "-e", "HOME", "-w", "/app", "-u", "1000:1000", "api", "sh", "-c", "echo hi")

		assert.Zero(t, status, h.Err.String())
		assert.Equal(t, "typed", h.Out.String())

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, dockerVM.UUID, requests[0].UUID)
		assert.Equal(t, streamtest.ExecPayload{
			Command: []string{"docker", "exec", "--interactive", "--tty", "--env", "A=b", "--env", "HOME=/home/me", "--workdir", "/app", "--user", "1000:1000", "abc123", "sh", "-c", "echo hi"},
			TTY:     true,
		}, payloadOf(t, requests[0]))
	})

	t.Run("without flags, only the container and the command", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(7) }}
		h, r, _ := harness(t, ingress, "")

		assert.Equal(t, 7, h.Run(newExecContainerCommand(r), "api", "false"))
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"docker", "exec", "abc123", "false"}}, payloadOf(t, ingress.Requests()[0]))
	})

	t.Run("what cannot be run is the error", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecContainerCommand(r), "-it", "api", "sh"))
		assert.Equal(t, "Error: the input device is not a TTY\n", h.Err.String())

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecContainerCommand(r), "-e", "=x", "api", "sh"))
		assert.Equal(t, "Error: invalid environment variable: =x\n", h.Err.String())

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecContainerCommand(r), "nope", "sh"))
		assert.Equal(t, "Error: no such container: nope\n", h.Err.String())
	})

	t.Run("a container and a command are required", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")

		assert.Equal(t, 2, h.Run(newExecContainerCommand(r), "api"))
	})
}
