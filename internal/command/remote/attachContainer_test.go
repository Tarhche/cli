package remote

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

// payloadOf is the exec payload request was asked with.
func payloadOf(t *testing.T, request streamtest.Request) streamtest.ExecPayload {
	t.Helper()

	var payload streamtest.ExecPayload
	require.NoError(t, json.Unmarshal([]byte(request.Payload), &payload))

	return payload
}

func TestAttachContainerCommand(t *testing.T) {
	t.Parallel()

	t.Run("docker attach in the container's vm, on a terminal when the local input is one", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			typed, _ := io.ReadAll(e.Stdin)
			_, _ = e.Stdout.Write(append([]byte("typed "), typed...))
			_ = e.Exit(0)
		}}

		h, r, terminal := harness(t, ingress, "keys")
		terminal.terminals[h.Cli.In] = true
		terminal.terminals[h.Cli.Out] = true

		status := h.Run(newAttachContainerCommand(r), "api")

		assert.Zero(t, status, h.Err.String())
		assert.Equal(t, "typed keys", h.Out.String())

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, dockerVM.UUID, requests[0].UUID)
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"docker", "attach", "abc123"}, TTY: true, Rows: 24, Cols: 80}, payloadOf(t, requests[0]))

		raw, restored := terminal.modes()
		assert.Equal(t, 1, raw)
		assert.Equal(t, 1, restored)
	})

	t.Run("docker attach's flags are its own, and the container's status is the cli's", func(t *testing.T) {
		t.Parallel()

		ended := make(chan bool, 1)

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = io.ReadAll(e.Stdin)
			ended <- e.StdinEnded()
			_ = e.Exit(2)
		}}

		h, r, terminal := harness(t, ingress, "ignored")
		terminal.terminals[h.Cli.In] = true

		status := h.Run(newAttachContainerCommand(r), "--detach-keys", "ctrl-x,y", "--no-stdin", "--sig-proxy=false", "api")

		assert.Equal(t, 2, status)
		assert.True(t, <-ended)

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"docker", "attach", "--detach-keys", "ctrl-x,y", "--no-stdin", "--sig-proxy=false", "abc123"}}, payloadOf(t, requests[0]))
	})

	t.Run("a container that cannot be found is the error", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")

		assert.Equal(t, 1, h.Run(newAttachContainerCommand(r), "nope"))
		assert.Equal(t, "Error: no such container: nope\n", h.Err.String())
	})

	t.Run("a container is required, and only one", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 2, h.Run(newAttachContainerCommand(r)))

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 2, h.Run(newAttachContainerCommand(r), "api", "other"))
	})
}
