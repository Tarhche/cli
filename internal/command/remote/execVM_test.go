package remote

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestExecVMCommand(t *testing.T) {
	t.Parallel()

	t.Run("a command's output, errors and status are the cli's, its input ended at once", func(t *testing.T) {
		t.Parallel()

		ended := make(chan bool, 1)

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = io.ReadAll(e.Stdin)
			ended <- e.StdinEnded()

			_, _ = e.Stdout.Write([]byte("out"))
			_, _ = e.Stderr.Write([]byte("err"))
			_ = e.Exit(3)
		}}

		h, r, _ := harness(t, ingress, "not read")

		status := h.Run(newExecVMCommand(r), "-e", "A=b", "-e", "HOME", "--env", "UNSET", "-w", "/tmp", "machine", "ls", "-la")

		assert.Equal(t, 3, status)
		assert.Equal(t, "out", h.Out.String())
		assert.Equal(t, "err", h.Err.String())
		assert.True(t, <-ended)

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, machine.UUID, requests[0].UUID)
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"ls", "-la"}, Env: []string{"A=b", "HOME=/home/me"}, WorkingDir: "/tmp"}, payloadOf(t, requests[0]))
	})

	t.Run("an interactive command reads the input, to its end", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			input, _ := io.ReadAll(e.Stdin)
			_, _ = e.Stdout.Write(bytes.ToUpper(input))
			_ = e.Exit(0)
		}}, "hello")

		assert.Zero(t, h.Run(newExecVMCommand(r), "-i", "machine", "tr", "a-z", "A-Z"), h.Err.String())
		assert.Equal(t, "HELLO", h.Out.String())
	})

	t.Run("on a terminal it is sized as the local one, and resized with it", func(t *testing.T) {
		t.Parallel()

		var local *fakeTerminal

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			local.resize(50, 150)

			size := <-e.Resizes
			_, _ = e.Stdout.Write([]byte{byte(size.Rows), byte(size.Cols)})
			_ = e.Exit(0)
		}}

		h, r, terminal := harness(t, ingress, "")
		terminal.terminals[h.Cli.In] = true
		terminal.terminals[h.Cli.Out] = true
		local = terminal

		assert.Zero(t, h.Run(newExecVMCommand(r), "-it", "machine", "sh"), h.Err.String())
		assert.Equal(t, []byte{50, 150}, h.Out.Bytes())
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"sh"}, TTY: true, Rows: 24, Cols: 80}, payloadOf(t, ingress.Requests()[0]))

		raw, restored := terminal.modes()
		assert.Equal(t, 1, raw)
		assert.Equal(t, 1, restored)
	})

	t.Run("a terminal for input that is not one is refused", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{}
		h, r, _ := harness(t, ingress, "")

		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "-it", "machine", "sh"))
		assert.Equal(t, "Error: the input device is not a TTY\n", h.Err.String())
		assert.Empty(t, ingress.Requests())
	})

	t.Run("a terminal whose output is not one is not sized", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(0) }}
		h, r, _ := harness(t, ingress, "")

		assert.Zero(t, h.Run(newExecVMCommand(r), "-t", "machine", "sh"), h.Err.String())
		assert.Equal(t, streamtest.ExecPayload{Command: []string{"sh"}, TTY: true}, payloadOf(t, ingress.Requests()[0]))
	})

	t.Run("a command killed by a signal exits with 255, saying so", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(-1) }}, "")

		assert.Equal(t, 255, h.Run(newExecVMCommand(r), "machine", "sleep", "9"))
		assert.Equal(t, "the command was killed by a signal\n", h.Err.String())
	})

	t.Run("a command that could not be waited for is the error", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Fail("the vm went away") }}, "")

		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "machine", "sleep", "9"))
		assert.Equal(t, "Error: the command could not be waited for: the vm went away\n", h.Err.String())
	})

	t.Run("what cannot be run is the error", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "-e", "=x", "machine", "true"))
		assert.Equal(t, "Error: invalid environment variable: =x\n", h.Err.String())

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "nope", "true"))
		assert.Equal(t, "Error: no such vm: nope\n", h.Err.String())

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "machine", "true"))
		assert.Equal(t, "Error: cannot open vms/0190-machine/exec: it is not there, or not yours (no such stream)\n", h.Err.String())

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		withoutIngress(t, h)
		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "machine", "true"))

		h, r, _ = harness(t, &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			_, _ = io.ReadAll(e.Stdin)
			_ = e.Drop()
		}}, "")
		assert.Equal(t, 1, h.Run(newExecVMCommand(r), "machine", "true"))
		assert.Equal(t, "Error: the stream ended before the command's exit was said\n", h.Err.String())
	})

	t.Run("a vm and a command are required", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")

		assert.Equal(t, 2, h.Run(newExecVMCommand(r), "machine"))
		assert.Contains(t, h.Err.String(), `"tarhche exec" requires at least 2 arguments.`)
	})
}
