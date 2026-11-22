package desktop_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/stream/streamtest"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const (
	machineUUID = "019a0000-0000-7000-8000-000000000001"
	dockUUID    = "019a0000-0000-7000-8000-000000000002"
)

// reachable is a Cli whose API knows a machine, a Docker VM and a container
// in it, and whose workload ingress is ingress.
func reachable(t *testing.T, ingress *streamtest.Ingress) *command.Cli {
	t.Helper()

	api := workloadtest.New().
		On(http.MethodGet, "/api/dashboard/my/workload/vms/"+machineUUID, http.StatusOK, workloadtest.VM(machineUUID, "machine", false, "running")).
		On(http.MethodGet, "/api/dashboard/my/workload/vms/"+dockUUID, http.StatusOK, workloadtest.VM(dockUUID, "dock", true, "running")).
		On(http.MethodGet, "/api/dashboard/my/workload/containers", http.StatusOK, workloadtest.Page(workloadtest.Container("c-1", "abc123", "web", "running", dockUUID, "dock")))

	server := httptest.NewServer(ingress)
	t.Cleanup(server.Close)

	h := commandtest.New(t, api)
	h.Cli.Globals.Ingress = server.URL

	return h.Cli
}

func listening(t *testing.T) (string, func() net.Conn) {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	return listener.Addr().String(), nil
}

func roundTripThrough(t *testing.T, address string) string {
	t.Helper()

	conn, err := new(net.Dialer).DialContext(context.Background(), "tcp", address)
	require.NoError(t, err)

	defer conn.Close()

	_, err = conn.Write([]byte("ping"))
	require.NoError(t, err)

	answer := make([]byte, len("ping"))
	_, err = io.ReadFull(conn, answer)
	require.NoError(t, err)

	return string(answer)
}

func TestStreamsOpen(t *testing.T) {
	t.Parallel()

	t.Run("a vm's own shell, told its size first and again when it changes", func(t *testing.T) {
		t.Parallel()

		sizes := make(chan streamtest.Control, 2)
		ingress := &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			for range 2 {
				message, err := terminal.Next()
				if err != nil || message.Control == nil {
					return
				}

				sizes <- *message.Control
			}

			_ = terminal.Output([]byte("hi"))
			_ = terminal.End()
		}}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}, Size: desktop.TerminalSize{Rows: 24, Cols: 80}})
		require.NoError(t, err)

		require.NoError(t, terminal.Resize(desktop.TerminalSize{Rows: 40, Cols: 120}))

		output, err := io.ReadAll(terminal)
		require.NoError(t, err)

		assert.Equal(t, "hi", string(output))
		assert.Equal(t, streamtest.Control{Type: "resize", Rows: 24, Cols: 80}, <-sizes)
		assert.Equal(t, streamtest.Control{Type: "resize", Rows: 40, Cols: 120}, <-sizes)
		assert.Equal(t, "attach", ingress.Requests()[0].Action)
		assert.NoError(t, terminal.Close())
	})

	t.Run("a command on a terminal in a vm, ending with the status it exited with", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			if strings.Join(e.Payload.Command, " ") != "top -b" || !e.Payload.TTY || e.Payload.Rows != 24 || e.Payload.Cols != 80 {
				_ = e.Fail("asked wrongly")

				return
			}

			_ = e.Send(1, []byte("load"))
			_ = e.Exit(3)
		}}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}, Command: []string{"top", "-b"}, Size: desktop.TerminalSize{Rows: 24, Cols: 80}})
		require.NoError(t, err)

		require.NoError(t, terminal.Resize(desktop.TerminalSize{Rows: 30, Cols: 100}))

		output, err := io.ReadAll(terminal)

		assert.Equal(t, "load", string(output))
		assert.Equal(t, command.StatusError{Code: 3}, err)

		_, err = terminal.Write([]byte("q"))
		assert.Error(t, err, "nothing reads what is typed into a command that ended")
		assert.NoError(t, terminal.Close())
	})

	t.Run("a command that exits with 0 ends", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(0) }}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}, Command: []string{"true"}})
		require.NoError(t, err)

		output, err := io.ReadAll(terminal)

		require.NoError(t, err)
		assert.Empty(t, output)
	})

	t.Run("a command that cannot be waited for says why", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Fail("lost") }}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}, Command: []string{"true"}})
		require.NoError(t, err)

		_, err = io.ReadAll(terminal)

		assert.ErrorContains(t, err, "lost")
	})

	t.Run("a shell in a container, through docker exec in its vm", func(t *testing.T) {
		t.Parallel()

		commands := make(chan []string, 1)
		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			commands <- e.Payload.Command
			_ = e.Exit(0)
		}}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: dockUUID, Container: "web"}})
		require.NoError(t, err)

		_, err = io.ReadAll(terminal)
		require.NoError(t, err)

		command := <-commands
		assert.Equal(t, []string{"docker", "exec", "-i", "-t", "abc123", "sh", "-c"}, command[:7])
		assert.Contains(t, command[7], "exec bash")
		assert.Equal(t, dockUUID, ingress.Requests()[0].UUID)
	})

	t.Run("a command in a container", func(t *testing.T) {
		t.Parallel()

		commands := make(chan []string, 1)
		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			commands <- e.Payload.Command
			_ = e.Exit(0)
		}}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: dockUUID, Container: "web"}, Command: []string{"psql"}})
		require.NoError(t, err)

		_, err = io.ReadAll(terminal)
		require.NoError(t, err)
		assert.Equal(t, []string{"docker", "exec", "-i", "-t", "abc123", "psql"}, <-commands)
	})

	t.Run("what cannot be opened is an error", func(t *testing.T) {
		t.Parallel()

		refusing := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) {
			return http.StatusServiceUnavailable, "not reachable: the vm is not running"
		}}

		cases := map[string]desktop.TerminalRequest{
			"a vm nothing names":            {Target: desktop.Target{VM: "missing"}},
			"a container nothing names":     {Target: desktop.Target{VM: dockUUID, Container: "missing"}},
			"a vm's shell refused":          {Target: desktop.Target{VM: machineUUID}},
			"a vm's command refused":        {Target: desktop.Target{VM: machineUUID}, Command: []string{"ls"}},
			"a container's command refused": {Target: desktop.Target{VM: dockUUID, Container: "web"}},
		}

		for name, request := range cases {
			_, err := desktop.NewStreams().Open(context.Background(), reachable(t, refusing), request)

			assert.Error(t, err, name)
		}
	})

	t.Run("what is typed reaches a vm's shell", func(t *testing.T) {
		t.Parallel()

		typed := make(chan string, 1)
		ingress := &streamtest.Ingress{Attach: func(terminal *streamtest.Terminal) {
			for {
				message, err := terminal.Next()
				if err != nil {
					return
				}

				if message.Input != nil {
					typed <- string(message.Input)
					_ = terminal.End()

					return
				}
			}
		}}

		terminal, err := desktop.NewStreams().Open(context.Background(), reachable(t, ingress), desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}, Size: desktop.TerminalSize{Rows: 24, Cols: 80}})
		require.NoError(t, err)

		_, err = terminal.Write([]byte("ls\n"))
		require.NoError(t, err)

		assert.Equal(t, "ls\n", <-typed)
		assert.NoError(t, terminal.Close())
	})

	t.Run("an api that cannot be reached names nothing", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		cli := command.NewCli(nil, nil, nil, command.WithConfig(h.Config), command.WithTokens(commandtest.Tokens(commandtest.Token)))
		cli.Globals.Host = "not a url"

		_, err := desktop.NewStreams().Open(context.Background(), cli, desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}})

		assert.Error(t, err)
	})

	t.Run("a context with no ingress opens nothing", func(t *testing.T) {
		t.Parallel()

		cli := reachable(t, &streamtest.Ingress{})
		cli.Globals.Ingress = "not a url"

		_, err := desktop.NewStreams().Open(context.Background(), cli, desktop.TerminalRequest{Target: desktop.Target{VM: machineUUID}})

		assert.Error(t, err)
	})
}

func TestStreamsForward(t *testing.T) {
	t.Parallel()

	t.Run("a port a vm publishes, until it is closed", func(t *testing.T) {
		t.Parallel()

		echo, _ := listening(t)
		ingress := &streamtest.Ingress{Forward: func(r *streamtest.Raw) { streamtest.Proxy(r, echo) }}

		forward, err := desktop.NewStreams().Forward(context.Background(), reachable(t, ingress), desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 8080})
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(forward.Address(), "127.0.0.1:"), forward.Address())
		assert.Equal(t, "ping", roundTripThrough(t, forward.Address()))
		assert.NoError(t, forward.Close())
		assert.NoError(t, forward.Wait())
		assert.JSONEq(t, `{"port":8080}`, ingress.Requests()[0].Payload)
	})

	t.Run("a container's port, relayed from inside its vm to its address", func(t *testing.T) {
		t.Parallel()

		echo, _ := listening(t)
		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			if e.Payload.Command[0] == "docker" {
				_ = e.Send(1, []byte("172.18.0.5\n"))
				_ = e.Exit(0)

				return
			}

			if strings.Join(e.Payload.Command, " ") != "nc 172.18.0.5 80" {
				_ = e.Fail("not nc")

				return
			}

			streamtest.Relay(e, echo)
		}}

		forward, err := desktop.NewStreams().Forward(context.Background(), reachable(t, ingress), desktop.ForwardRequest{Target: desktop.Target{VM: dockUUID, Container: "web"}, Port: 80})
		require.NoError(t, err)

		assert.Equal(t, "ping", roundTripThrough(t, forward.Address()))
		assert.NoError(t, forward.Close())
	})

	t.Run("on the local port asked for", func(t *testing.T) {
		t.Parallel()

		free, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)

		port := free.Addr().(*net.TCPAddr).Port
		require.NoError(t, free.Close())

		forward, err := desktop.NewStreams().Forward(context.Background(), reachable(t, &streamtest.Ingress{}), desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 8080, LocalPort: uint16(port)})
		require.NoError(t, err)

		defer forward.Close()

		assert.Equal(t, free.Addr().String(), forward.Address())
	})

	t.Run("until what it was forwarded for is done", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		forward, err := desktop.NewStreams().Forward(ctx, reachable(t, &streamtest.Ingress{}), desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 8080})
		require.NoError(t, err)

		cancel()

		stopped := make(chan error, 1)
		go func() { stopped <- forward.Wait() }()

		select {
		case err := <-stopped:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("the forward outlived what it was forwarded for")
		}
	})

	t.Run("what cannot be forwarded is an error", func(t *testing.T) {
		t.Parallel()

		inspectFails := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(1) }}

		cases := map[string]struct {
			ingress *streamtest.Ingress
			request desktop.ForwardRequest
		}{
			"a port a machine does not publish": {ingress: &streamtest.Ingress{}, request: desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 9090}},
			"a vm nothing names":                {ingress: &streamtest.Ingress{}, request: desktop.ForwardRequest{Target: desktop.Target{VM: "missing"}, Port: 8080}},
			"a container nothing names":         {ingress: &streamtest.Ingress{}, request: desktop.ForwardRequest{Target: desktop.Target{VM: dockUUID, Container: "missing"}, Port: 80}},
			"a container with no address":       {ingress: inspectFails, request: desktop.ForwardRequest{Target: desktop.Target{VM: dockUUID, Container: "web"}, Port: 80}},
		}

		for name, c := range cases {
			_, err := desktop.NewStreams().Forward(context.Background(), reachable(t, c.ingress), c.request)

			assert.Error(t, err, name)
		}

		cli := reachable(t, &streamtest.Ingress{})
		cli.Globals.Ingress = "not a url"

		_, err := desktop.NewStreams().Forward(context.Background(), cli, desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 8080})
		assert.Error(t, err, "a context with no ingress")
	})

	t.Run("a local port that is taken cannot be listened on", func(t *testing.T) {
		t.Parallel()

		taken, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)

		defer taken.Close()

		_, err = desktop.NewStreams().Forward(context.Background(), reachable(t, &streamtest.Ingress{}), desktop.ForwardRequest{Target: desktop.Target{VM: machineUUID}, Port: 8080, LocalPort: uint16(taken.Addr().(*net.TCPAddr).Port)})

		assert.Error(t, err)
	})
}

func TestStreamsCopy(t *testing.T) {
	t.Parallel()

	running := &streamtest.Ingress{Exec: streamtest.Run}

	t.Run("a file copied out of a vm and back in", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		remote := filepath.Join(dir, "remote")
		local := filepath.Join(dir, "local")
		require.NoError(t, os.MkdirAll(remote, 0o750))
		require.NoError(t, os.MkdirAll(local, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(remote, "hello.txt"), []byte("hello"), 0o600))

		streams := desktop.NewStreams()
		cli := reachable(t, running)

		require.NoError(t, streams.CopyOut(context.Background(), cli, desktop.CopyRequest{Target: desktop.Target{VM: machineUUID}, Path: filepath.Join(remote, "hello.txt"), LocalPath: filepath.Join(local, "copied.txt")}))

		copied, err := os.ReadFile(filepath.Join(local, "copied.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(copied))

		require.NoError(t, streams.CopyIn(context.Background(), cli, desktop.CopyRequest{Target: desktop.Target{VM: machineUUID}, Path: remote, LocalPath: filepath.Join(local, "copied.txt")}))

		back, err := os.ReadFile(filepath.Join(remote, "copied.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(back))
	})

	t.Run("a container's files are copied by docker cp in its vm", func(t *testing.T) {
		t.Parallel()

		commands := make(chan []string, 4)
		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			commands <- e.Payload.Command
			_ = e.Exit(1)
		}}

		err := desktop.NewStreams().CopyOut(context.Background(), reachable(t, ingress), desktop.CopyRequest{Target: desktop.Target{VM: dockUUID, Container: "web"}, Path: "/etc/hosts", LocalPath: t.TempDir()})

		assert.Error(t, err, "a copy docker cp failed")
		assert.Equal(t, []string{"docker", "cp"}, (<-commands)[:2])
		assert.Equal(t, dockUUID, ingress.Requests()[0].UUID)
	})

	t.Run("what cannot be reached is not copied", func(t *testing.T) {
		t.Parallel()

		streams := desktop.NewStreams()

		cases := map[string]desktop.Target{
			"a vm nothing names":        {VM: "missing"},
			"a container nothing names": {VM: dockUUID, Container: "missing"},
		}

		for name, target := range cases {
			assert.Error(t, streams.CopyIn(context.Background(), reachable(t, running), desktop.CopyRequest{Target: target, Path: "/tmp", LocalPath: t.TempDir()}), name)
			assert.Error(t, streams.CopyOut(context.Background(), reachable(t, running), desktop.CopyRequest{Target: target, Path: "/tmp", LocalPath: t.TempDir()}), name)
		}

		cli := reachable(t, running)
		cli.Globals.Ingress = "not a url"

		assert.Error(t, streams.CopyOut(context.Background(), cli, desktop.CopyRequest{Target: desktop.Target{VM: machineUUID}, Path: "/tmp", LocalPath: t.TempDir()}), "a context with no ingress")
	})
}
