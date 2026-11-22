package desktop_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/desktop"
)

func TestNewOpener(t *testing.T) {
	t.Parallel()

	newBase := func(t *testing.T) (*command.Cli, *config.File, *auth.Manager) {
		t.Helper()

		cfg, err := config.Load(t.TempDir())
		require.NoError(t, err)

		base := command.NewCli(nil, &bytes.Buffer{}, &bytes.Buffer{})
		base.Globals = command.Globals{Context: "local", Host: "http://127.0.0.1:9000", Ingress: "http://127.0.0.1:9030", Language: "fa", Debug: true}

		return base, cfg, auth.New(cfg, nil)
	}

	t.Run("the context the app was opened on keeps what the global flags lay over it", func(t *testing.T) {
		t.Parallel()

		base, cfg, manager := newBase(t)

		cli := desktop.NewOpener(base, cfg, manager, "local")("local")

		assert.Equal(t, command.Globals{Context: "local", Host: "http://127.0.0.1:9000", Ingress: "http://127.0.0.1:9030", Language: "fa", Debug: true}, cli.Globals)
	})

	t.Run("another context is talked to as it is", func(t *testing.T) {
		t.Parallel()

		base, cfg, manager := newBase(t)

		cli := desktop.NewOpener(base, cfg, manager, "local")("other")

		assert.Equal(t, command.Globals{Context: "other", Language: "fa", Debug: true}, cli.Globals)
	})

	t.Run("every Cli shares the configuration and the sessions", func(t *testing.T) {
		t.Parallel()

		base, cfg, manager := newBase(t)

		cli := desktop.NewOpener(base, cfg, manager, "local")("other")

		gotConfig, err := cli.Config()
		require.NoError(t, err)

		gotAuth, err := cli.Auth()
		require.NoError(t, err)

		assert.Same(t, cfg, gotConfig)
		assert.Same(t, manager, gotAuth)
		assert.Same(t, base.Out, cli.Out)
	})
}

func TestWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("its Cli talks to the context it was opened on", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)

		name, err := w.Cli().ContextName()

		require.NoError(t, err)
		assert.Equal(t, testContext, name)
	})

	t.Run("its client reaches the context's API, signed in", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{}`)
		w, _ := newWorkspace(t, api)

		client, err := w.Client(context.Background())
		require.NoError(t, err)
		require.NoError(t, client.Get(context.Background(), "/api/ping", nil, nil))

		assert.Equal(t, request{Method: http.MethodGet, Path: "/api/ping", Token: "Bearer " + commandtest.Token}, api.last(t))
	})

	t.Run("its context is the one it talks to", func(t *testing.T) {
		t.Parallel()

		w, h := newWorkspace(t, nil)

		current, err := w.Context()
		require.NoError(t, err)

		name, err := w.ContextName()
		require.NoError(t, err)

		assert.Equal(t, config.Context{Name: testContext, API: h.Server.URL, Ingress: h.Server.URL}, current)
		assert.Equal(t, testContext, name)
	})

	t.Run("reading sees the configuration and the context it talks to", func(t *testing.T) {
		t.Parallel()

		w, h := newWorkspace(t, nil)

		err := w.Read(func(cfg *config.File, current string) error {
			assert.Same(t, h.Config, cfg)
			assert.Equal(t, testContext, current)

			return nil
		})

		assert.NoError(t, err)
	})

	t.Run("reading says what the reader said", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		failure := errors.New("failure")

		err := w.Read(func(*config.File, string) error { return failure })

		assert.ErrorIs(t, err, failure)
	})

	t.Run("a change is saved, and the context it answers talked to", func(t *testing.T) {
		t.Parallel()

		w, h := newWorkspace(t, nil)

		err := w.Change(func(cfg *config.File, current string) (string, error) {
			assert.Equal(t, testContext, current)

			return "other", cfg.SetContext(config.Context{Name: "other", API: "https://other.example"})
		})
		require.NoError(t, err)

		saved, err := config.Load(h.Config.Dir())
		require.NoError(t, err)

		_, err = saved.Context("other")
		assert.NoError(t, err)

		name, err := w.ContextName()
		require.NoError(t, err)
		assert.Equal(t, "other", name)
	})

	t.Run("a change that answers no context keeps talking to the same one", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		before := w.Cli()

		require.NoError(t, w.Change(func(*config.File, string) (string, error) { return "", nil }))

		assert.Same(t, before, w.Cli())
	})

	t.Run("a change that fails is not saved", func(t *testing.T) {
		t.Parallel()

		w, h := newWorkspace(t, nil)
		failure := errors.New("failure")

		err := w.Change(func(*config.File, string) (string, error) { return "other", failure })

		assert.ErrorIs(t, err, failure)
		assert.NoFileExists(t, h.Config.Path())

		name, nameErr := w.ContextName()
		require.NoError(t, nameErr)
		assert.Equal(t, testContext, name)
	})

	t.Run("a change that cannot be saved fails", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "config")

		cfg, err := config.Load(dir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(dir, nil, 0o600))

		w := desktop.NewWorkspace(testContext, func(contextName string) *command.Cli {
			cli := command.NewCli(nil, io.Discard, io.Discard, command.WithConfig(cfg))
			cli.Globals.Context = contextName

			return cli
		})

		assert.Error(t, w.Change(func(*config.File, string) (string, error) { return "", nil }))
	})

	t.Run("a configuration that cannot be read can be neither read nor changed", func(t *testing.T) {
		t.Parallel()

		w := brokenWorkspace(t)

		assert.Error(t, w.Read(func(*config.File, string) error { return nil }))
		assert.Error(t, w.Change(func(*config.File, string) (string, error) { return "", nil }))
	})

	t.Run("a configuration that cannot be read is no context to talk to", func(t *testing.T) {
		t.Parallel()

		w := brokenWorkspace(t)

		_, nameErr := w.ContextName()
		_, contextErr := w.Context()

		assert.Error(t, nameErr)
		assert.Error(t, contextErr)
		assert.Error(t, w.Reopen())
	})

	t.Run("a configuration that cannot be read is not changed though the context is known", func(t *testing.T) {
		t.Parallel()

		dir := brokenConfig(t)
		w := desktop.NewWorkspace(testContext, func(contextName string) *command.Cli {
			cli := command.NewCli(nil, io.Discard, io.Discard)
			cli.Globals.ConfigDir = dir
			cli.Globals.Context = contextName

			return cli
		})

		assert.Error(t, w.Change(func(*config.File, string) (string, error) { return "", nil }))
	})

	t.Run("reopening opens the context it talks to afresh", func(t *testing.T) {
		t.Parallel()

		w, _ := newWorkspace(t, nil)
		before := w.Cli()

		require.NoError(t, w.Reopen())

		name, err := w.ContextName()
		require.NoError(t, err)

		assert.NotSame(t, before, w.Cli())
		assert.Equal(t, testContext, name)
	})

	t.Run("what its session may do is what its token says its roles allow", func(t *testing.T) {
		t.Parallel()

		granted, err := newWorkspaceGranting(t, nil, "articles.index", "self.workload.vms.exec").Granted(context.Background())

		require.NoError(t, err)
		assert.Equal(t, access.Granted{"articles.index", "self.workload.vms.exec"}, granted)
	})

	t.Run("a session that cannot be had may do nothing", func(t *testing.T) {
		t.Parallel()

		_, sessionErr := workspaceFailing(t, nil, auth.ErrNotLoggedIn).Granted(context.Background())
		_, configErr := brokenWorkspace(t).Granted(context.Background())

		assert.ErrorIs(t, sessionErr, auth.ErrNotLoggedIn)
		assert.Error(t, configErr)
	})
}
