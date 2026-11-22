package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

func TestAttachVMCommand(t *testing.T) {
	t.Parallel()

	t.Run("a terminal on the vm: the local one raw, the remote one sized as it is and resized with it", func(t *testing.T) {
		t.Parallel()

		said := make(chan []streamtest.Message, 1)

		var local *fakeTerminal

		ingress := &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			var messages []streamtest.Message

			for len(messages) < 3 {
				message, err := terminal.Next()
				if err != nil {
					break
				}

				messages = append(messages, message)

				if len(messages) == 2 {
					local.resize(40, 120)
				}
			}

			said <- messages

			_ = terminal.Output([]byte("file\r\n"))
			_ = terminal.End()
		}}

		h, r, terminal := harness(t, ingress, "ls\n")
		terminal.terminals[h.Cli.In] = true
		terminal.terminals[h.Cli.Out] = true
		local = terminal

		status := h.Run(newAttachVMCommand(r), "machine")

		assert.Zero(t, status, h.Err.String())
		assert.Equal(t, "file\r\n", h.Out.String())
		assert.Equal(t, []streamtest.Message{
			{Control: &streamtest.Control{Type: "resize", Rows: 24, Cols: 80}},
			{Input: []byte("ls\n")},
			{Control: &streamtest.Control{Type: "resize", Rows: 40, Cols: 120}},
		}, <-said)

		raw, restored := terminal.modes()
		assert.Equal(t, 1, raw)
		assert.Equal(t, 1, restored)

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, "vms", requests[0].Kind)
		assert.Equal(t, machine.UUID, requests[0].UUID)
		assert.Equal(t, "attach", requests[0].Action)
		assert.Empty(t, requests[0].Payload)
	})

	t.Run("input that is not a terminal is typed as it is, and ended as a terminal's is", func(t *testing.T) {
		t.Parallel()

		said := make(chan []streamtest.Message, 1)

		h, r, terminal := harness(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			var messages []streamtest.Message

			for len(messages) < 2 {
				message, err := terminal.Next()
				if err != nil {
					break
				}

				messages = append(messages, message)
			}

			said <- messages

			_ = terminal.End()
		}}, "echo hi\n")

		status := h.Run(newAttachVMCommand(r), "machine")

		assert.Zero(t, status, h.Err.String())
		assert.Equal(t, []streamtest.Message{{Input: []byte("echo hi\n")}, {Input: []byte{endOfTransmission}}}, <-said)

		raw, _ := terminal.modes()
		assert.Zero(t, raw)
	})

	t.Run("an interrupt lets go of the terminal", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		h, r, _ := harness(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			cancel()

			for {
				if _, err := terminal.Next(); err != nil {
					return
				}
			}
		}}, "")

		assert.Equal(t, 130, h.RunContext(ctx, newAttachVMCommand(r), "machine"))
	})

	t.Run("a terminal that cannot be made raw is the error", func(t *testing.T) {
		t.Parallel()

		h, r, terminal := harness(t, &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			for {
				if _, err := terminal.Next(); err != nil {
					return
				}
			}
		}}, "")
		terminal.terminals[h.Cli.In] = true
		terminal.rawErr = errors.New("cannot make it raw")

		assert.Equal(t, 1, h.Run(newAttachVMCommand(r), "machine"))
		assert.Equal(t, "Error: cannot make it raw\n", h.Err.String())
	})

	t.Run("what cannot be attached to is the error", func(t *testing.T) {
		t.Parallel()

		refused := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return 503, "not reachable: the vm is not running" }}

		h, r, _ := harness(t, refused, "")
		assert.Equal(t, 1, h.Run(newAttachVMCommand(r), "machine"))
		assert.Equal(t, "Error: cannot open vms/0190-machine/attach: it cannot be reached now (not reachable: the vm is not running)\n", h.Err.String())

		h, r, _ = harness(t, refused, "")
		assert.Equal(t, 1, h.Run(newAttachVMCommand(r), "nope"))
		assert.Equal(t, "Error: no such vm: nope\n", h.Err.String())

		h, r, _ = harness(t, refused, "")
		withoutIngress(t, h)
		assert.Equal(t, 1, h.Run(newAttachVMCommand(r), "machine"))
		assert.Equal(t, "Error: the context \"test\" names no workload ingress: give it one, or name one with --ingress\n", h.Err.String())
	})

	t.Run("a vm is required, and only one", func(t *testing.T) {
		t.Parallel()

		h, r, _ := harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 2, h.Run(newAttachVMCommand(r)))

		h, r, _ = harness(t, &streamtest.Ingress{}, "")
		assert.Equal(t, 2, h.Run(newAttachVMCommand(r), "machine", "other"))
		assert.Contains(t, h.Err.String(), `"tarhche attach" requires exactly 1 argument.`)
	})
}
