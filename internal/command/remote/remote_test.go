package remote

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/stream/streamtest"
	"github.com/Tarhche/cli/internal/workload"
)

var (
	// machine is a VM that runs no dockerd, and publishes 8080.
	machine = workload.VM{UUID: "0190-machine", Name: "machine", Slug: "machine-x1", State: "running", Ports: []uint{8080}}

	// dockerVM is a Docker VM, which publishes 8080.
	dockerVM = workload.VM{UUID: "0190-docker", Name: "dock", Slug: "dock-x2", Docker: true, State: "running", Ports: []uint{8080}}

	// api is a container in dockerVM.
	api = workload.Container{UUID: "0190-api", DockerID: "abc123", Name: "api", State: "running", VM: dockerVM}
)

// resolver finds machine, dockerVM and api by their names.
type resolver struct{}

func (resolver) VM(_ context.Context, reference string) (workload.VM, error) {
	for _, vm := range []workload.VM{machine, dockerVM} {
		if vm.Name == reference {
			return vm, nil
		}
	}

	return workload.VM{}, fmt.Errorf("no such vm: %s", reference)
}

func (resolver) Container(_ context.Context, reference string) (workload.Container, error) {
	if reference == api.Name {
		return api, nil
	}

	return workload.Container{}, fmt.Errorf("no such container: %s", reference)
}

// fakeTerminal is a local terminal whose streams are terminals when a test
// says so, of the size it says, resized when it says.
type fakeTerminal struct {
	terminals map[any]bool
	rawErr    error
	resizes   chan struct{}

	lock     sync.Mutex
	rows     uint
	cols     uint
	raw      int
	restored int
}

func newFakeTerminal(terminals ...any) *fakeTerminal {
	t := &fakeTerminal{terminals: map[any]bool{}, resizes: make(chan struct{}, 1), rows: 24, cols: 80}

	for _, stream := range terminals {
		t.terminals[stream] = true
	}

	return t
}

func (t *fakeTerminal) IsTerminal(stream any) bool {
	return t.terminals[stream]
}

func (t *fakeTerminal) MakeRaw(any, any) (func() error, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.rawErr != nil {
		return nil, t.rawErr
	}

	t.raw++

	return func() error {
		t.lock.Lock()
		defer t.lock.Unlock()

		t.restored++

		return nil
	}, nil
}

func (t *fakeTerminal) Size(any) (uint, uint, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.rows == 0 {
		return 0, 0, errNotTerminal
	}

	return t.rows, t.cols, nil
}

// resize makes the terminal rows by cols, and says it was resized.
func (t *fakeTerminal) resize(rows, cols uint) {
	t.lock.Lock()
	t.rows, t.cols = rows, cols
	t.lock.Unlock()

	t.resizes <- struct{}{}
}

func (t *fakeTerminal) Resized(ctx context.Context) <-chan struct{} {
	changes := make(chan struct{})

	go func() {
		defer close(changes)

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.resizes:
				select {
				case changes <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return changes
}

func (t *fakeTerminal) modes() (raw, restored int) {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.raw, t.restored
}

// syncBuffer is a buffer written and read from goroutines of their own.
type syncBuffer struct {
	lock   sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.buffer.String()
}

// harness is a CLI whose API and ingress are ingress, the commands of whose
// remote find what resolver finds and show terminals on a fake terminal.
func harness(t *testing.T, ingress *streamtest.Ingress, input string) (*commandtest.Harness, *remote, *fakeTerminal) {
	t.Helper()

	h := commandtest.New(t, ingress)
	h.Cli.In = strings.NewReader(input)

	terminal := newFakeTerminal()
	r := newRemote(h.Cli, WithResolver(resolver{}))
	r.terminal = terminal
	r.lookupEnv = func(name string) (string, bool) {
		if name == "HOME" {
			return "/home/me", true
		}

		return "", false
	}

	return h, r, terminal
}

// harnessed is a harness with the remote of its commands, its terminal and
// its ingress.
type harnessed struct {
	*commandtest.Harness
	remote   *remote
	terminal *fakeTerminal
	ingress  *streamtest.Ingress
}

// withoutIngress makes the harness's context name no ingress.
func withoutIngress(t *testing.T, h *commandtest.Harness) {
	t.Helper()

	require.NoError(t, h.Config.SetContext(config.Context{Name: "test", API: h.Server.URL}))
}

func TestCommands(t *testing.T) {
	t.Parallel()

	t.Run("a container's and a vm's commands are a fresh set at each call", func(t *testing.T) {
		t.Parallel()

		cli := command.NewCli(nil, nil, nil)

		for _, commands := range [][]console.Command{ContainerCommands(cli), VMCommands(cli)} {
			var names []string
			for _, c := range commands {
				names = append(names, c.Name())
			}

			assert.Equal(t, []string{"attach", "exec", "cp", "port-forward"}, names)
		}

		containers, again := ContainerCommands(cli), ContainerCommands(cli)
		assert.NotSame(t, containers[0], again[0])

		vms, againVMs := VMCommands(cli), VMCommands(cli)
		assert.NotSame(t, vms[0], againVMs[0])
	})

	t.Run("every command says what it does and how it is asked for, as docker's does", func(t *testing.T) {
		t.Parallel()

		usages := map[string]string{
			"attach":       "attach ",
			"exec":         "exec [OPTIONS] ",
			"cp":           "cp [OPTIONS] ",
			"port-forward": "port-forward [OPTIONS] ",
		}

		h := commandtest.New(t, nil)

		for _, commands := range [][]console.Command{ContainerCommands(h.Cli), VMCommands(h.Cli)} {
			for _, c := range commands {
				h.Out.Reset()

				assert.Zero(t, h.Run(c, "--help"))
				assert.Contains(t, h.Out.String(), c.Description()+"\n\nUsage:\n\n  "+c.Usage()+"\n", c.Name())
				assert.True(t, strings.HasPrefix(c.Usage(), usages[c.Name()]), c.Usage())
			}
		}
	})

	t.Run("a resolver given is the one references are found by", func(t *testing.T) {
		t.Parallel()

		r := newRemote(command.NewCli(nil, nil, nil), WithResolver(resolver{}))

		found, err := r.resolve(context.Background())

		require.NoError(t, err)
		assert.Equal(t, resolver{}, found)
	})

	t.Run("without one, references are found through the api", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		found, err := newRemote(h.Cli).resolve(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("a context that cannot be had is the error", func(t *testing.T) {
		t.Parallel()

		cli := command.NewCli(nil, nil, nil, command.WithConfig(emptyConfig(t)))
		cli.Globals.Context = "missing"

		r := newRemote(cli)

		_, err := r.resolve(context.Background())
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.vm(context.Background(), "machine")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.container(context.Background(), "api")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.dialer()
		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("streams are signed with the session's tokens, or fail as they do", func(t *testing.T) {
		t.Parallel()

		cfg := emptyConfig(t)
		require.NoError(t, cfg.SetContext(config.Context{Name: "local", API: "http://127.0.0.1:1", Ingress: "http://127.0.0.1:2"}))
		require.NoError(t, cfg.Use("local"))

		cli := command.NewCli(nil, nil, nil, command.WithConfig(cfg))

		_, tokensErr := cli.Tokens()
		dialer, err := newRemote(cli).dialer()

		if tokensErr != nil {
			assert.EqualError(t, err, tokensErr.Error())
		} else {
			require.NoError(t, err)
			assert.NotNil(t, dialer)
		}
	})
}

func emptyConfig(t *testing.T) *config.File {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	return cfg
}
