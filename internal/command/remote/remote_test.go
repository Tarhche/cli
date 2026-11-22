package remote

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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

	// theirs is somebody else's Docker VM, which publishes 8080.
	theirs = workload.VM{UUID: "0190-theirs", Name: "theirs", Slug: "theirs-x3", Docker: true, State: "running", Ports: []uint{8080}}

	// cache is a container in theirs.
	cache = workload.Container{UUID: "0190-cache", DockerID: "def456", Name: "cache", State: "running", VM: theirs}
)

// resolver finds machine, dockerVM and api by their names among one's own,
// and theirs and cache as well among anybody's.
type resolver struct {
	audience workload.Audience
}

// resolverOf is the resolver of audience's workloads.
func resolverOf(audience workload.Audience) workload.Resolver {
	return resolver{audience: audience}
}

func (r resolver) VM(_ context.Context, reference string) (workload.VM, error) {
	vms := []workload.VM{machine, dockerVM}
	if r.audience == workload.Everyone {
		vms = append(vms, theirs)
	}

	for _, vm := range vms {
		if vm.Name == reference {
			return vm, nil
		}
	}

	return workload.VM{}, fmt.Errorf("no such vm: %s", reference)
}

func (r resolver) Container(_ context.Context, reference string) (workload.Container, error) {
	containers := []workload.Container{api}
	if r.audience == workload.Everyone {
		containers = append(containers, cache)
	}

	for _, container := range containers {
		if container.Name == reference {
			return container, nil
		}
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
	r := newRemote(h.Cli, WithResolver(resolverOf))
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
				assert.Contains(t, h.Out.String(), "-A, --all-users", c.Name())
				assert.True(t, strings.HasPrefix(c.Usage(), usages[c.Name()]), c.Usage())
			}
		}
	})

	t.Run("a resolver given is the one references are found by, for whose workloads they are among", func(t *testing.T) {
		t.Parallel()

		r := newRemote(command.NewCli(nil, nil, nil), WithResolver(resolverOf))

		for _, audience := range []workload.Audience{workload.Mine, workload.Everyone} {
			found, err := r.resolve(context.Background(), audience)

			require.NoError(t, err)
			assert.Equal(t, resolver{audience: audience}, found)
		}
	})

	t.Run("without one, references are found through the api: one's own routes, or the administrator's", func(t *testing.T) {
		t.Parallel()

		const uuid = "01900000-0000-7000-8000-000000000001"

		routes := map[workload.Audience]string{
			workload.Mine:     "/api/dashboard/my/workload/vms/" + uuid,
			workload.Everyone: "/api/dashboard/workload/vms/" + uuid,
		}

		for audience, route := range routes {
			var (
				lock  sync.Mutex
				asked []string
			)

			h := commandtest.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				lock.Lock()
				asked = append(asked, r.URL.Path)
				lock.Unlock()

				http.NotFound(w, r)
			}))

			found, err := newRemote(h.Cli).resolve(context.Background(), audience)
			require.NoError(t, err)

			_, err = found.VM(context.Background(), uuid)
			require.Error(t, err)

			lock.Lock()
			assert.Equal(t, route, asked[0], route)
			lock.Unlock()
		}
	})

	t.Run("a context that cannot be had is the error", func(t *testing.T) {
		t.Parallel()

		cli := command.NewCli(nil, nil, nil, command.WithConfig(emptyConfig(t)))
		cli.Globals.Context = "missing"

		r := newRemote(cli)

		_, err := r.resolve(context.Background(), workload.Mine)
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.vm(context.Background(), workload.Mine, "machine")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.container(context.Background(), workload.Everyone, "api")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = r.dialer(workload.Mine)
		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("streams are signed with the session's tokens, or fail as they do", func(t *testing.T) {
		t.Parallel()

		cfg := emptyConfig(t)
		require.NoError(t, cfg.SetContext(config.Context{Name: "local", API: "http://127.0.0.1:1", Ingress: "http://127.0.0.1:2"}))
		require.NoError(t, cfg.Use("local"))

		cli := command.NewCli(nil, nil, nil, command.WithConfig(cfg))

		_, tokensErr := cli.Tokens()
		dialer, err := newRemote(cli).dialer(workload.Mine)

		if tokensErr != nil {
			assert.EqualError(t, err, tokensErr.Error())
		} else {
			require.NoError(t, err)
			assert.NotNil(t, dialer)
		}
	})

	t.Run("with -A a reference is found among anybody's, and its streams opened on it; without, among one's own", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			command string
			args    []string
			stream  string
			missing string
		}{
			{command: "vm attach", args: []string{"theirs"}, stream: "vms/0190-theirs/attach", missing: "no such vm: theirs"},
			{command: "vm exec", args: []string{"theirs", "true"}, stream: "vms/0190-theirs/exec", missing: "no such vm: theirs"},
			{command: "vm cp", args: []string{"theirs:/etc/hosts", "-"}, stream: "vms/0190-theirs/exec", missing: "no such vm: theirs"},
			{command: "container attach", args: []string{"cache"}, stream: "vms/0190-theirs/exec", missing: "no such container: cache"},
			{command: "container exec", args: []string{"cache", "true"}, stream: "vms/0190-theirs/exec", missing: "no such container: cache"},
			{command: "container cp", args: []string{"cache:/etc/hosts", "-"}, stream: "vms/0190-theirs/exec", missing: "no such container: cache"},
			{command: "container port-forward", args: []string{"cache", "80"}, stream: "vms/0190-theirs/exec", missing: "no such container: cache"},
		}

		for _, c := range cases {
			t.Run(c.command, func(t *testing.T) {
				t.Parallel()

				unreachable := func() *streamtest.Ingress {
					return &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return http.StatusServiceUnavailable, "not running" }}
				}

				h, commands := commandsOf(t, unreachable())
				assert.Equal(t, 1, h.Run(commands[c.command], append([]string{"-A"}, c.args...)...))
				assert.Contains(t, h.Err.String(), "cannot open "+c.stream+": it cannot be reached now (not running)\n")

				h, commands = commandsOf(t, unreachable())
				assert.Equal(t, 1, h.Run(commands[c.command], c.args...))
				assert.Equal(t, "Error: "+c.missing+"\n", h.Err.String())
			})
		}
	})

	t.Run("a stream refused for the roles says the permission it needs: one's own vm's, or anybody's with -A", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			command    string
			args       []string
			permission string
			doing      string
		}{
			{command: "vm attach", args: []string{"machine"}, permission: "workload.vms.attach"},
			{command: "vm exec", args: []string{"machine", "true"}, permission: "workload.vms.exec"},
			{command: "vm cp", args: []string{"machine:/etc/hosts", "-"}, permission: "workload.vms.exec"},
			{command: "container attach", args: []string{"api"}, permission: "workload.vms.exec"},
			{command: "container exec", args: []string{"api", "true"}, permission: "workload.vms.exec"},
			{command: "container cp", args: []string{"api:/etc/hosts", "-"}, permission: "workload.vms.exec"},
			{command: "container port-forward", args: []string{"api", "80"}, permission: "workload.vms.exec", doing: "finding the address of container api: "},
		}

		forbidding := func() *streamtest.Ingress {
			return &streamtest.Ingress{Refuse: func(streamtest.Request) (int, string) { return http.StatusForbidden, "forbidden" }}
		}

		for _, c := range cases {
			t.Run(c.command, func(t *testing.T) {
				t.Parallel()

				h, commands := commandsOf(t, forbidding())
				assert.Equal(t, 1, h.Run(commands[c.command], c.args...))
				assert.Equal(t, "Error: "+c.doing+`permission denied: this needs the "self.`+c.permission+`" permission, which none of your roles gives you`+"\n", h.Err.String())

				h, commands = commandsOf(t, forbidding())
				assert.Equal(t, 1, h.Run(commands[c.command], append([]string{"-A"}, c.args...)...))
				assert.Equal(t, "Error: "+c.doing+`permission denied: this needs the "`+c.permission+`" permission, which none of your roles gives you`+"\n", h.Err.String())
			})
		}
	})
}

// commandsOf is a harness whose ingress is ingress, and the commands of a vm
// and of a container run in it, by group and name ("vm exec"), which find
// what resolverOf finds.
func commandsOf(t *testing.T, ingress *streamtest.Ingress) (*commandtest.Harness, map[string]console.Command) {
	t.Helper()

	h := commandtest.New(t, ingress)
	commands := make(map[string]console.Command)

	for group, set := range map[string][]console.Command{
		"vm":        VMCommands(h.Cli, WithResolver(resolverOf)),
		"container": ContainerCommands(h.Cli, WithResolver(resolverOf)),
	} {
		for _, c := range set {
			commands[group+" "+c.Name()] = c
		}
	}

	return h, commands
}

func emptyConfig(t *testing.T) *config.File {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	return cfg
}
