package workload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/workload"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

// failing is a writer that cannot be written to.
type failing struct{}

func (failing) Write([]byte) (int, error) {
	return 0, errors.New("closed")
}

func TestFlagValues(t *testing.T) {
	t.Parallel()

	t.Run("a list is given once for each value", func(t *testing.T) {
		t.Parallel()

		var l list
		require.NoError(t, l.Set("a"))
		require.NoError(t, l.Set("b=c"))

		assert.Equal(t, list{"a", "b=c"}, l)
		assert.Equal(t, "a,b=c", l.String())
		assert.Equal(t, "list", l.Type())
	})

	t.Run("ports are given once each or several at once", func(t *testing.T) {
		t.Parallel()

		var p portList
		require.NoError(t, p.Set("80"))
		require.NoError(t, p.Set("443, 8080"))

		assert.Equal(t, portList{80, 443, 8080}, p)
		assert.Equal(t, "80,443,8080", p.String())
		assert.Equal(t, "ports", p.Type())
		assert.EqualError(t, p.Set("http"), `"http" is not a port`)
		assert.EqualError(t, p.Set("65536"), `"65536" is not a port`)
	})

	t.Run("a new vm is described only when a flag says something of it", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, (&newVMFlags{}).described())
		assert.Equal(t, &workload.NewDockerVM{Name: "box"}, (&newVMFlags{name: "box"}).described())
		assert.Equal(t, &workload.NewDockerVM{Network: &workload.Network{Ingress: workload.Deny}}, (&newVMFlags{ingress: workload.Deny}).described())
	})

	t.Run("a usage error names the command", func(t *testing.T) {
		t.Parallel()

		flags := console.NewFlagSet("tarhche vm ls", nil)

		assert.Equal(t, command.UsageError{Command: "tarhche vm ls", Message: "bad 1"}, usageError(flags, "bad %d", 1))
	})
}

func TestClientOf(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	cli := command.NewCli(nil, &bytes.Buffer{}, &bytes.Buffer{}, command.WithConfig(cfg), command.WithAuth(auth.New(cfg, nil)))
	cli.Globals.Context = "missing"

	_, err = clientOf(context.Background(), cli, false)

	assert.ErrorIs(t, err, config.ErrNoSuchContext)
}

func TestMomentOf(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)

	for value, want := range map[string]time.Time{
		"":                          {},
		"42m":                       now.Add(-42 * time.Minute),
		"1h30m":                     now.Add(-90 * time.Minute),
		"2026-11-22T09:00:00Z":      time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC),
		"2026-11-22T09:00:00+03:30": time.Date(2026, 11, 22, 5, 30, 0, 0, time.UTC),
		"2026-11-22T09:00:00":       time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local),
		"2026-11-22T09:00":          time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local),
		"2026-11-22":                time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local),
		"1700000000":                time.Unix(1700000000, 0),
		"1700000000.5":              time.Unix(1700000000, 500_000_000),
	} {
		got, err := momentOf(value, now)

		require.NoError(t, err, value)
		assert.True(t, want.Equal(got), "%q is %v, not %v", value, got, want)
	}

	for _, value := range []string{"soon", "Inf", "NaN", "2026-13-01"} {
		_, err := momentOf(value, now)

		assert.Error(t, err, value)
	}
}

func TestOutput(t *testing.T) {
	t.Parallel()

	t.Run("what is inspected is written whole, or through a template", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, writeInspected(&out, []json.RawMessage{json.RawMessage(`{"a":1}`)}, ""))
		assert.Equal(t, "[\n    {\n        \"a\": 1\n    }\n]\n", out.String())

		assert.Error(t, writeInspected(&out, []json.RawMessage{json.RawMessage(`{`)}, "{{.a}}"))
		assert.Error(t, writeInspected(&out, []json.RawMessage{json.RawMessage(`{"a":1}`)}, "{{.a"))
	})

	t.Run("what refuses everything stops a command given several references", func(t *testing.T) {
		t.Parallel()

		assert.True(t, stops(context.Canceled))
		assert.True(t, stops(auth.ErrNotLoggedIn))
		assert.True(t, stops(&api.Error{StatusCode: 401}))
		assert.True(t, stops(&api.Error{StatusCode: 403}))
		assert.False(t, stops(&api.Error{StatusCode: 400}))
		assert.False(t, stops(&workload.NotFoundError{Kind: "vm", Reference: "web"}))
	})

	t.Run("an output that cannot be written to fails what writes to it", func(t *testing.T) {
		t.Parallel()

		cli := command.NewCli(nil, failing{}, &bytes.Buffer{})

		assert.EqualError(t, eachReference(cli, []string{"a"}, func(string) error { return nil }), "closed")
		assert.EqualError(t, inspectEach(cli, []string{"a"}, "", func(string) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }), "closed")
		assert.EqualError(t, warnRemoval(failing{}, "web", workload.Removal{Dependents: []workload.Dependent{{Kind: "stack", UUID: "s1"}}}), "closed")
		assert.EqualError(t, sayMadeVM(failing{}, workload.ChosenVM{Created: true}), "closed")
	})

	t.Run("what stops a command stops it before what it was given next", func(t *testing.T) {
		t.Parallel()

		var acted []string

		cli := command.NewCli(nil, &bytes.Buffer{}, &bytes.Buffer{})
		err := eachReference(cli, []string{"a", "b"}, func(reference string) error {
			acted = append(acted, reference)

			return auth.ErrNotLoggedIn
		})

		assert.ErrorIs(t, err, auth.ErrNotLoggedIn)
		assert.Equal(t, []string{"a"}, acted)

		err = inspectEach(cli, []string{"a", "b"}, "", func(string) (json.RawMessage, error) { return nil, context.Canceled })
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("nothing is said of a removal nothing needs, nor of a vm not made", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, warnRemoval(failing{}, "web", workload.Removal{}))
		assert.NoError(t, sayMadeVM(failing{}, workload.ChosenVM{}))
	})

	t.Run("ids, text and owners as listings show them", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "0123456789ab", truncated("0123456789abcdef", false))
		assert.Equal(t, "0123456789abcdef", truncated("0123456789abcdef", true))
		assert.Equal(t, "abc", truncated("abc", false))
		assert.Equal(t, "abcd", ellipsis("abcd", 4))
		assert.Equal(t, "abc…", ellipsis("abcde", 4))
		assert.Equal(t, "u1", ownerOf(nil, "u1"))
		assert.Equal(t, "u2", ownerOf(&workload.Owner{UUID: "u2"}, "u1"))
	})
}

func TestSpecs(t *testing.T) {
	t.Parallel()

	t.Run("ports to publish as docker writes them", func(t *testing.T) {
		t.Parallel()

		for spec, want := range map[string]workload.PortBinding{
			"80":                {ContainerPort: 80},
			"8080:80":           {ContainerPort: 80, HostPort: 8080},
			"8080:80/udp":       {ContainerPort: 80, HostPort: 8080, Protocol: "udp"},
			"127.0.0.1:8080:80": {ContainerPort: 80, HostPort: 8080, HostIP: "127.0.0.1"},
			"127.0.0.1::80":     {ContainerPort: 80, HostIP: "127.0.0.1"},
			"[::1]:8080:80/tcp": {ContainerPort: 80, HostPort: 8080, HostIP: "::1", Protocol: "tcp"},
		} {
			got, err := publishedOf(spec)

			require.NoError(t, err, spec)
			assert.Equal(t, want, got, spec)
		}

		for _, spec := range []string{"", "http", "x:80", "[::1", "[::1]:1:2:3", "1:2:3:4", "8000-8010:80", "70000"} {
			_, err := publishedOf(spec)

			assert.Error(t, err, spec)
		}
	})

	t.Run("volumes to mount as docker writes them", func(t *testing.T) {
		t.Parallel()

		for spec, want := range map[string]workload.Mount{
			"/data":           {Type: workload.MountVolume, Target: "/data"},
			"/data:ro":        {Type: workload.MountVolume, Target: "/data", ReadOnly: true},
			"data:/data":      {Type: workload.MountVolume, Source: "data", Target: "/data"},
			"./site:/srv:rw":  {Type: workload.MountBind, Source: "./site", Target: "/srv"},
			"~/x:/x:readonly": {Type: workload.MountBind, Source: "~/x", Target: "/x", ReadOnly: true},
		} {
			got, err := volumeOf(spec)

			require.NoError(t, err, spec)
			assert.Equal(t, want, got, spec)
		}

		for _, spec := range []string{"a:b:c:d", "data:/data:z"} {
			_, err := volumeOf(spec)

			assert.Error(t, err, spec)
		}
	})

	t.Run("mounts as docker's --mount writes them", func(t *testing.T) {
		t.Parallel()

		for spec, want := range map[string]workload.Mount{
			"type=tmpfs,destination=/run":     {Type: workload.MountTmpfs, Target: "/run"},
			"source=data,target=/data,ro=1":   {Type: workload.MountVolume, Source: "data", Target: "/data", ReadOnly: true},
			"src=d,dst=/d,readonly=false":     {Type: workload.MountVolume, Source: "d", Target: "/d"},
			"Type=bind,Source=/etc,Target=/e": {Type: workload.MountBind, Source: "/etc", Target: "/e"},
		} {
			got, err := mountOf(spec)

			require.NoError(t, err, spec)
			assert.Equal(t, want, got, spec)
		}

		for _, spec := range []string{"bogus", "ro=perhaps"} {
			_, err := mountOf(spec)

			assert.Error(t, err, spec)
		}
	})

	t.Run("a tmpfs takes no options", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, workload.Mount{Type: workload.MountTmpfs, Target: "/run"}, tmpfsOf("/run:size=64m,mode=1777"))
	})
}

func TestRows(t *testing.T) {
	t.Parallel()

	t.Run("a container says what it is doing as docker says it, else in its kind's words", func(t *testing.T) {
		t.Parallel()

		of := func(state, dockerStatus, reason string, failure *workload.Failure) workload.ContainerManifest {
			var c workload.ContainerManifest
			c.Status.State, c.Status.DockerStatus, c.Status.Reason, c.Status.Failure = state, dockerStatus, reason, failure

			return c
		}

		assert.Equal(t, "Up 2 hours", containerStatusOf(of("running", "Up 2 hours", "", nil)))
		assert.Equal(t, "waiting: its vm is stopped", containerStatusOf(of("waiting", "Up 2 hours", "its vm is stopped", nil)))
		assert.Equal(t, "failed: in use", containerStatusOf(of("failed", "", "x", &workload.Failure{Message: "in use"})))
		assert.Equal(t, "running", containerStatusOf(of("running", "", "started", nil)))
		assert.Equal(t, "missing", containerStatusOf(of("missing", "", "", nil)))
	})

	t.Run("an image's reference is its repository, tag and digest", func(t *testing.T) {
		t.Parallel()

		for reference, want := range map[string][3]string{
			"":                          {"<none>", "<none>", ""},
			"nginx":                     {"nginx", "latest", ""},
			"localhost:5000/app:2":      {"localhost:5000/app", "2", ""},
			"ghcr.io/acme/app@sha256:1": {"ghcr.io/acme/app", "<none>", "sha256:1"},
		} {
			repository, tag, digest := splitReference(reference)

			assert.Equal(t, want, [3]string{repository, tag, digest}, reference)
		}

		assert.Equal(t, "i1", imageIDOf(workload.ImageManifest{Metadata: workload.Metadata{UUID: "i1"}}, false))

		asked := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
		pulling := workload.Placed[workload.ImageManifest]{
			VM:   workload.VMManifest{Metadata: workload.Metadata{UUID: "v1"}},
			Item: workload.ImageManifest{Metadata: workload.Metadata{UUID: "i2", CreatedAt: asked}, Spec: workload.ImageSpec{Reference: "redis"}},
		}

		row := imageRowOf(pulling, false)
		assert.Equal(t, asked, row.CreatedAt, "an image docker has not pulled yet was created when it was asked for")
		assert.Equal(t, "v1", row.VM, "a vm with no name is told by its uuid")
		assert.Equal(t, "redis", row.Repository)
	})

	t.Run("a vm given no name is given one after what it is", func(t *testing.T) {
		t.Parallel()

		assert.Regexp(t, `^machine-[a-z2-7]{4}$`, madeUpName(workload.KindMachine, ""))
		assert.Regexp(t, `^docker-[a-z2-7]{4}$`, madeUpName(workload.KindDocker, "ubuntu"))
		assert.Regexp(t, `^app-[a-z2-7]{4}$`, madeUpName(workload.KindMachine, "localhost:5000/acme/app:2"))
	})

	t.Run("shares and percents as docker's stats shows them", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "33.33%", percent(share(1, 3)))
		assert.Equal(t, "0.00%", percent(share(1, 0)))
		assert.Equal(t, strconv.FormatFloat(math.Round(12.5*100)/100, 'f', 2, 64)+"%", percent(12.5))
	})
}

func TestLogFollower(t *testing.T) {
	t.Parallel()

	t.Run("a line that cannot be written stops it", func(t *testing.T) {
		t.Parallel()

		follower := &logFollower{
			read: func(context.Context, workload.LogsQuery) (workload.Logs, error) {
				return workload.Logs{Items: []workload.LogLine{{Line: "a"}}}, nil
			},
			write: lineWriter(failing{}, failing{}, true),
			every: time.Millisecond,
		}

		assert.EqualError(t, follower.run(context.Background(), workload.LogsQuery{}, false, false), "closed")

		follower.write = lineWriter(failing{}, failing{}, false)
		assert.EqualError(t, follower.run(context.Background(), workload.LogsQuery{}, false, false), "closed")
	})

	t.Run("a line that cannot be written while following stops it", func(t *testing.T) {
		t.Parallel()

		reads := 0
		follower := &logFollower{
			read: func(context.Context, workload.LogsQuery) (workload.Logs, error) {
				reads++
				if reads == 1 {
					return workload.Logs{}, nil
				}

				return workload.Logs{Items: []workload.LogLine{{At: time.Now(), Line: "a"}}}, nil
			},
			write: lineWriter(failing{}, failing{}, false),
			every: time.Millisecond,
		}

		assert.EqualError(t, follower.run(context.Background(), workload.LogsQuery{}, false, true), "closed")
	})

	t.Run("what cannot be read at first is an error", func(t *testing.T) {
		t.Parallel()

		follower := &logFollower{
			read: func(context.Context, workload.LogsQuery) (workload.Logs, error) {
				return workload.Logs{}, errors.New("gone")
			},
		}

		assert.EqualError(t, follower.run(context.Background(), workload.LogsQuery{}, false, true), "gone")
	})

	t.Run("an interrupt stops the wait", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.ErrorIs(t, wait(ctx, time.Hour), context.Canceled)
	})
}

// cliWriting is a Cli whose API is fake, writing to out and errOut.
func cliWriting(t *testing.T, fake *workloadtest.API, out, errOut io.Writer) *command.Cli {
	t.Helper()

	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	client, err := api.New(server.URL, api.WithTokens(commandtest.Tokens(commandtest.Token)))
	require.NoError(t, err)

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	return command.NewCli(strings.NewReader(""), out, errOut, command.WithConfig(cfg), command.WithTokens(commandtest.Tokens(commandtest.Token)), command.WithClient(client))
}

// runOn runs cmd with args through a console writing to cli's streams.
func runOn(ctx context.Context, cli *command.Cli, cmd console.Command, args ...string) int {
	return console.New(command.Name, "", cli.Out, cli.Err).Register(cmd).Run(ctx, append([]string{command.Name, cmd.Name()}, args...))
}

// cancelling is an output that ends a context once it is written to.
type cancelling struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (c *cancelling) Write(p []byte) (int, error) {
	defer c.cancel()

	return c.Buffer.Write(p)
}

func TestCommandsWhoseOutputFails(t *testing.T) {
	t.Parallel()

	madeVM := workloadtest.Object{"vm": workloadtest.Object{"uuid": vmWeb, "name": "web", "created": true}}

	t.Run("a vm made for a container or a stack that cannot be said fails it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodPost, everyone+"/containers", http.StatusCreated, madeVM.With("container", nginx)).
			On(http.MethodPost, everyone+"/stacks", http.StatusCreated, madeVM.With("stack", blog))

		cli := cliWriting(t, fake, &bytes.Buffer{}, failing{})

		assert.Equal(t, console.ExitFailure, runOn(context.Background(), cli, newRunContainerCommand(cli), "-d", "nginx"))
		assert.Equal(t, console.ExitFailure, runOn(context.Background(), cli, newDeployStackCommand(cli), "-c", "-", "blog"))
	})

	t.Run("ports that cannot be written fail it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx))
		cli := cliWriting(t, fake, failing{}, &bytes.Buffer{})

		assert.Equal(t, console.ExitFailure, runOn(context.Background(), cli, newShowContainerPortsCommand(cli), "nginx"))
		assert.Equal(t, console.ExitFailure, runOn(context.Background(), cli, newShowContainerPortsCommand(cli), "nginx", "80"))
	})

	t.Run("stats are shown on a cleared terminal until interrupted while waiting", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/stats", http.StatusOK, `{"pids":1}`)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		out := &cancelling{cancel: cancel}
		cli := cliWriting(t, fake, out, &bytes.Buffer{})

		cmd := newShowContainerStatsCommand(cli)
		cmd.isTerminal = func(any) bool { return true }

		assert.Equal(t, command.ExitInterrupted, runOn(ctx, cli, cmd))
		assert.True(t, strings.HasPrefix(out.String(), clearScreen), out.String())
	})

	t.Run("a terminal that cannot be cleared fails it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/containers/c-nginx/stats", http.StatusOK, `{"pids":1}`)
		cli := cliWriting(t, fake, failing{}, &bytes.Buffer{})

		cmd := newShowContainerStatsCommand(cli)
		cmd.isTerminal = func(any) bool { return true }

		assert.Equal(t, console.ExitFailure, runOn(context.Background(), cli, cmd))
	})
}

func TestLogFollowerInterrupted(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	follower := &logFollower{
		read: func(context.Context, workload.LogsQuery) (workload.Logs, error) {
			cancel()

			return workload.Logs{}, nil
		},
		write: lineWriter(&bytes.Buffer{}, &bytes.Buffer{}, false),
		every: time.Hour,
	}

	assert.ErrorIs(t, follower.run(ctx, workload.LogsQuery{}, false, true), context.Canceled)
}
