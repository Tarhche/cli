package reach

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/stream/streamtest"
	"github.com/Tarhche/cli/internal/workload"
)

var (
	machine = workload.VM{UUID: "vm-1", Name: "machine", Ports: []uint{8080}}
	dock    = workload.VM{UUID: "vm-2", Name: "dock", Docker: true, Ports: []uint{8080}}
	web     = workload.Container{UUID: "c-1", DockerID: "abc123", Name: "web", VM: dock}
)

func dialerOf(t *testing.T, ingress *streamtest.Ingress) *stream.Dialer {
	t.Helper()

	server := httptest.NewServer(ingress)
	t.Cleanup(server.Close)

	dialer, err := stream.NewDialer(server.URL, commandtest.Tokens(streamtest.Token))
	require.NoError(t, err)

	return dialer
}

func echoServer(t *testing.T) string {
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

	return listener.Addr().String()
}

func roundTrip(t *testing.T, conn io.ReadWriteCloser) string {
	t.Helper()

	defer conn.Close()

	_, err := conn.Write([]byte("ping"))
	require.NoError(t, err)

	answer := make([]byte, len("ping"))
	_, err = io.ReadFull(conn, answer)
	require.NoError(t, err)

	return string(answer)
}

func relaying(echo string, command *[]string) func(*streamtest.Exec) {
	return func(e *streamtest.Exec) {
		*command = e.Payload.Command
		streamtest.Relay(e, echo)
	}
}

func actions(ingress *streamtest.Ingress) []string {
	var asked []string
	for _, request := range ingress.Requests() {
		asked = append(asked, request.Action)
	}

	return asked
}

func TestVMPort(t *testing.T) {
	t.Parallel()

	t.Run("a machine's port it does not publish has nothing to relay it", func(t *testing.T) {
		t.Parallel()

		_, err := VMPort(dialerOf(t, &streamtest.Ingress{}), machine, "machine", 9090)

		assert.EqualError(t, err, "vm machine does not publish port 9090, and runs no dockerd to relay it from inside: publish the port to forward it")
	})

	t.Run("a port the vm publishes is reached through the forward action", func(t *testing.T) {
		t.Parallel()

		echo := echoServer(t)
		ingress := &streamtest.Ingress{Forward: func(r *streamtest.Raw) { streamtest.Proxy(r, echo) }}

		dial, err := VMPort(dialerOf(t, ingress), machine, "machine", 8080)
		require.NoError(t, err)

		conn, err := dial(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "ping", roundTrip(t, conn))
		require.Len(t, ingress.Requests(), 1)
		assert.Equal(t, "forward", ingress.Requests()[0].Action)
		assert.JSONEq(t, `{"port":8080}`, ingress.Requests()[0].Payload)
	})

	t.Run("a port a docker vm does not publish is relayed by nc inside it", func(t *testing.T) {
		t.Parallel()

		var command []string
		ingress := &streamtest.Ingress{Exec: relaying(echoServer(t), &command)}

		dial, err := VMPort(dialerOf(t, ingress), dock, "dock", 9090)
		require.NoError(t, err)

		conn, err := dial(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "ping", roundTrip(t, conn))
		assert.Equal(t, []string{"nc", "127.0.0.1", "9090"}, command)
		assert.Equal(t, []string{"exec"}, actions(ingress))
	})

	t.Run("a docker vm whose forward is refused is relayed to, from then on", func(t *testing.T) {
		t.Parallel()

		var command []string
		ingress := &streamtest.Ingress{
			Exec: relaying(echoServer(t), &command),
			Refuse: func(request streamtest.Request) (int, string) {
				if request.Action == "forward" {
					return http.StatusNotFound, "no such vm"
				}

				return 0, ""
			},
		}

		dial, err := VMPort(dialerOf(t, ingress), dock, "dock", 8080)
		require.NoError(t, err)

		for range 2 {
			conn, err := dial(context.Background())
			require.NoError(t, err)

			assert.Equal(t, "ping", roundTrip(t, conn))
		}

		assert.Equal(t, []string{"forward", "exec", "exec"}, actions(ingress))
		assert.Equal(t, []string{"nc", "127.0.0.1", "8080"}, command)
	})

	t.Run("a machine whose forward is refused does not publish the port, or denies ingress", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return http.StatusNotFound, "no such vm" }}

		dial, err := VMPort(dialerOf(t, ingress), machine, "machine", 8080)
		require.NoError(t, err)

		_, err = dial(context.Background())

		assert.ErrorContains(t, err, "vm machine does not publish port 8080, or its ingress is denied")
		assert.True(t, stream.IsNotFound(err))
	})

	t.Run("any other refusal is that refusal", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) {
			return http.StatusServiceUnavailable, "not reachable: the vm is not running"
		}}

		dial, err := VMPort(dialerOf(t, ingress), dock, "dock", 8080)
		require.NoError(t, err)

		_, err = dial(context.Background())

		assert.True(t, stream.IsUnavailable(err))
		assert.Equal(t, []string{"forward"}, actions(ingress))
	})

	t.Run("a docker vm whose forward is forbidden is not relayed to: the permission to forward is missing", func(t *testing.T) {
		t.Parallel()

		var command []string
		ingress := &streamtest.Ingress{
			Exec: relaying(echoServer(t), &command),
			Refuse: func(request streamtest.Request) (int, string) {
				if request.Action == "forward" {
					return http.StatusForbidden, "forbidden"
				}

				return 0, ""
			},
		}

		dial, err := VMPort(dialerOf(t, ingress), dock, "dock", 8080)
		require.NoError(t, err)

		for range 2 {
			_, err = dial(context.Background())

			assert.True(t, stream.IsForbidden(err))
		}

		assert.Equal(t, []string{"forward", "forward"}, actions(ingress))
		assert.Empty(t, command)
	})
}

func TestContainerAddress(t *testing.T) {
	t.Parallel()

	inspecting := func(output string, code int, command *[]string) *streamtest.Ingress {
		return &streamtest.Ingress{Exec: func(e *streamtest.Exec) {
			*command = e.Payload.Command
			_ = e.Send(1, []byte(output))
			_ = e.Exit(code)
		}}
	}

	t.Run("the address on the first network it is on, as its vm's dockerd says", func(t *testing.T) {
		t.Parallel()

		var command []string
		ingress := inspecting("172.18.0.5 10.0.0.2 \n", 0, &command)

		address, err := ContainerAddress(context.Background(), dialerOf(t, ingress), web, "web")

		require.NoError(t, err)
		assert.Equal(t, "172.18.0.5", address)
		assert.Equal(t, []string{"docker", "inspect", "--format", containerAddresses, "abc123"}, command)
		assert.Equal(t, dock.UUID, ingress.Requests()[0].UUID)
	})

	t.Run("a container on no network has no address", func(t *testing.T) {
		t.Parallel()

		var command []string

		_, err := ContainerAddress(context.Background(), dialerOf(t, inspecting(" \n", 0, &command)), web, "web")

		assert.EqualError(t, err, "container web has no address to forward to: is it running?")
	})

	t.Run("an inspection that fails says so", func(t *testing.T) {
		t.Parallel()

		var command []string

		_, err := ContainerAddress(context.Background(), dialerOf(t, inspecting("", 1, &command)), web, "web")

		assert.ErrorContains(t, err, "finding the address of container web")
	})
}

func TestContainerPort(t *testing.T) {
	t.Parallel()

	t.Run("a container's port is relayed by nc in its vm to its address", func(t *testing.T) {
		t.Parallel()

		var command []string
		ingress := &streamtest.Ingress{Exec: relaying(echoServer(t), &command)}

		conn, err := ContainerPort(dialerOf(t, ingress), web, "172.18.0.5", 80)(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "ping", roundTrip(t, conn))
		assert.Equal(t, []string{"nc", "172.18.0.5", "80"}, command)
		assert.Equal(t, dock.UUID, ingress.Requests()[0].UUID)
	})

	t.Run("a relay that cannot be opened is an error", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) {
			return http.StatusServiceUnavailable, "not reachable: the vm is not running"
		}}

		_, err := ContainerPort(dialerOf(t, ingress), web, "172.18.0.5", 80)(context.Background())

		assert.True(t, stream.IsUnavailable(err))
	})
}

func TestDialer(t *testing.T) {
	t.Parallel()

	t.Run("the ingress of the context, signed in with its session", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Exec: func(e *streamtest.Exec) { _ = e.Exit(0) }}
		server := httptest.NewServer(ingress)
		t.Cleanup(server.Close)

		h := commandtest.New(t, nil)
		h.Cli.Globals.Ingress = server.URL

		dialer, err := Dialer(h.Cli)
		require.NoError(t, err)

		require.NoError(t, dialer.Runner(machine.UUID).Run(context.Background(), []string{"true"}, nil, io.Discard))
		assert.Equal(t, []string{"bearer", streamtest.Token}, ingress.Requests()[0].Protocols)
		assert.Contains(t, ingress.Requests()[0].UserAgent, "tarhche-cli/")
	})

	t.Run("configured by the options given: a stream refused for the roles is explained", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return http.StatusForbidden, "forbidden" }}
		server := httptest.NewServer(ingress)
		t.Cleanup(server.Close)

		h := commandtest.New(t, nil)
		h.Cli.Globals.Ingress = server.URL

		dialer, err := Dialer(h.Cli, stream.WithExplainer(access.NewStreamExplainer(workload.Mine)))
		require.NoError(t, err)

		_, err = dialer.Attach(context.Background(), stream.KindVMs, machine.UUID)

		require.EqualError(t, err, `permission denied: this needs the "self.workload.vms.attach" permission, which none of your roles gives you`)
		assert.Contains(t, ingress.Requests()[0].UserAgent, "tarhche-cli/", "introduced as the cli all the same")
	})

	t.Run("a context that names no ingress", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		require.NoError(t, h.Config.SetContext(config.Context{Name: "bare", API: h.Server.URL}))
		h.Cli.Globals.Context = "bare"

		_, err := Dialer(h.Cli)

		assert.EqualError(t, err, `the context "bare" names no workload ingress: give it one, or name one with --ingress`)
	})

	t.Run("a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli.Globals.Context = "missing"

		_, err := Dialer(h.Cli)

		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("a session that cannot be had", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.CredentialsStore = "nowhere"

		_, err := Dialer(command.NewCli(nil, nil, nil, command.WithConfig(h.Config)))

		assert.ErrorContains(t, err, "nowhere")
	})
}
