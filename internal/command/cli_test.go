package command

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/config"
)

type staticTokens string

func (s staticTokens) Token(context.Context) (string, error) { return string(s), nil }

func (s staticTokens) Renew(context.Context, string) (string, error) { return string(s), nil }

func configIn(t *testing.T, contexts ...config.Context) *config.File {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	for _, c := range contexts {
		require.NoError(t, cfg.SetContext(c))
	}

	return cfg
}

func TestNewCli(t *testing.T) {
	t.Parallel()

	in, out, errOut := strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}

	cli := NewCli(in, out, errOut)

	assert.Same(t, in, cli.In)
	assert.Same(t, out, cli.Out)
	assert.Same(t, errOut, cli.Err)
}

func TestCliConfig(t *testing.T) {
	t.Parallel()

	t.Run("the configuration given is the one", func(t *testing.T) {
		t.Parallel()

		cfg := configIn(t)

		got, err := NewCli(nil, nil, nil, WithConfig(cfg)).Config()

		require.NoError(t, err)
		assert.Same(t, cfg, got)
	})

	t.Run("the configuration is read once from where the globals point", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{"current_context":"x","contexts":{"x":{"api":"https://x.example"}}}`), 0o600))

		cli := NewCli(nil, nil, nil)
		cli.Globals.ConfigDir = dir

		first, err := cli.Config()
		require.NoError(t, err)

		second, err := cli.Config()
		require.NoError(t, err)

		assert.Same(t, first, second)
		assert.Equal(t, "x", first.Current())
	})

	t.Run("a configuration that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{`), 0o600))

		cli := NewCli(nil, nil, nil)
		cli.Globals.ConfigDir = dir

		_, err := cli.Config()

		assert.Error(t, err)
	})
}

func TestCliContext(t *testing.T) {
	t.Parallel()

	local := config.Context{Name: "local", API: "http://localhost:8000", Ingress: "http://localhost:8030"}

	t.Run("the current context", func(t *testing.T) {
		t.Parallel()

		cfg := configIn(t, local)
		require.NoError(t, cfg.Use("local"))

		got, err := NewCli(nil, nil, nil, WithConfig(cfg)).Context()

		require.NoError(t, err)
		assert.Equal(t, local, got)
	})

	t.Run("the context the globals name, with their api and ingress over it", func(t *testing.T) {
		t.Parallel()

		cli := NewCli(nil, nil, nil, WithConfig(configIn(t, local)))
		cli.Globals.Context = "local"
		cli.Globals.Host = "http://127.0.0.1:9000"
		cli.Globals.Ingress = "http://127.0.0.1:9030"

		got, err := cli.Context()

		require.NoError(t, err)
		assert.Equal(t, config.Context{Name: "local", API: "http://127.0.0.1:9000", Ingress: "http://127.0.0.1:9030"}, got)
	})

	t.Run("a context that does not exist is an error", func(t *testing.T) {
		t.Parallel()

		cli := NewCli(nil, nil, nil, WithConfig(configIn(t)))
		cli.Globals.Context = "missing"

		_, err := cli.Context()

		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("a configuration that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{`), 0o600))

		cli := NewCli(nil, nil, nil)
		cli.Globals.ConfigDir = dir

		_, err := cli.Context()
		assert.Error(t, err)

		_, err = cli.ContextName()
		assert.Error(t, err)
	})
}

func TestCliAuth(t *testing.T) {
	t.Parallel()

	t.Run("the manager given is the one", func(t *testing.T) {
		t.Parallel()

		manager := auth.New(configIn(t), nil)

		got, err := NewCli(nil, nil, nil, WithAuth(manager)).Auth()

		require.NoError(t, err)
		assert.Same(t, manager, got)
	})

	t.Run("a configuration that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{`), 0o600))

		cli := NewCli(nil, nil, nil)
		cli.Globals.ConfigDir = dir

		_, err := cli.Auth()

		assert.Error(t, err)
	})
}

func TestCliTokens(t *testing.T) {
	t.Parallel()

	t.Run("the source given is the one", func(t *testing.T) {
		t.Parallel()

		got, err := NewCli(nil, nil, nil, WithTokens(staticTokens("t"))).Tokens()

		require.NoError(t, err)
		assert.Equal(t, staticTokens("t"), got)
	})

	t.Run("else the manager's source of the current context", func(t *testing.T) {
		t.Parallel()

		cfg := configIn(t)

		got, err := NewCli(nil, nil, nil, WithConfig(cfg), WithAuth(auth.New(cfg, nil))).Tokens()

		require.NoError(t, err)
		assert.NotNil(t, got)
	})
}

func TestCliClient(t *testing.T) {
	t.Parallel()

	t.Run("the client given is the one", func(t *testing.T) {
		t.Parallel()

		client, err := api.New("https://backend.example")
		require.NoError(t, err)

		got, err := NewCli(nil, nil, nil, WithClient(client)).Client(context.Background())

		require.NoError(t, err)
		assert.Same(t, client, got)
	})

	t.Run("else a client of the context's api, signed in, built once", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer t", r.Header.Get("Authorization"))
			assert.Equal(t, "fa", r.Header.Get(api.LanguageHeader))
			assert.Contains(t, r.Header.Get("User-Agent"), "tarhche-cli/")
		}))
		t.Cleanup(server.Close)

		cfg := configIn(t, config.Context{Name: "test", API: server.URL})
		require.NoError(t, cfg.Use("test"))

		cli := NewCli(nil, nil, nil, WithConfig(cfg), WithTokens(staticTokens("t")))
		cli.Globals.Language = "fa"

		client, err := cli.Client(context.Background())
		require.NoError(t, err)
		require.NoError(t, client.Get(context.Background(), "/", nil, nil))

		again, err := cli.Client(context.Background())
		require.NoError(t, err)
		assert.Same(t, client, again)
	})

	t.Run("a context that does not exist is an error", func(t *testing.T) {
		t.Parallel()

		cli := NewCli(nil, nil, nil, WithConfig(configIn(t)), WithTokens(staticTokens("t")))
		cli.Globals.Context = "missing"

		_, err := cli.Client(context.Background())

		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("an api that is no url is an error", func(t *testing.T) {
		t.Parallel()

		cli := NewCli(nil, nil, nil, WithConfig(configIn(t)), WithTokens(staticTokens("t")))
		cli.Globals.Host = "not a url"

		_, err := cli.Client(context.Background())

		assert.Error(t, err)
	})
}

func TestParseGlobals(t *testing.T) {
	t.Parallel()

	t.Run("the flags before the command are read", func(t *testing.T) {
		t.Parallel()

		var globals Globals
		ParseGlobals([]string{"--desktop", "-c", "local", "-D", "ps", "--version"}, &globals)

		assert.True(t, globals.Desktop)
		assert.True(t, globals.Debug)
		assert.Equal(t, "local", globals.Context)
		assert.False(t, globals.Version)
	})

	t.Run("what cannot be read is left alone", func(t *testing.T) {
		t.Parallel()

		var globals Globals
		ParseGlobals([]string{"--bogus", "--desktop"}, &globals)

		assert.False(t, globals.Desktop)
	})
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	assert.False(t, IsTerminal(&bytes.Buffer{}))
	assert.False(t, IsTerminal(nil))

	file, err := os.CreateTemp(t.TempDir(), "not-a-terminal")
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	assert.False(t, IsTerminal(file))
	assert.False(t, NewCli(file, nil, file).Interactive())
}

func TestCliExit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status console.ExitStatus
		said   string
	}{
		{name: "no error", err: nil, status: console.ExitSuccess},
		{name: "an interrupt", err: context.Canceled, status: ExitInterrupted},
		{name: "a usage error", err: UsageError{Command: "tarhche rm", Message: `"tarhche rm" requires at least 1 argument.`}, status: console.ExitUsageError, said: "tarhche: \"tarhche rm\" requires at least 1 argument.\nSee 'tarhche rm --help'.\n"},
		{name: "a status", err: StatusError{Code: 3}, status: 3},
		{name: "a status with a message", err: StatusError{Code: 4, Message: "it failed"}, status: 4, said: "it failed\n"},
		{name: "anything else", err: errors.New("it broke"), status: console.ExitFailure, said: "Error: it broke\n"},
		{name: "not logged in", err: auth.ErrNotLoggedIn, status: console.ExitFailure, said: "Error: not logged in to \"test\" (https://backend.example): run 'tarhche login' first\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			cfg := configIn(t, config.Context{Name: "test", API: "https://backend.example"})
			require.NoError(t, cfg.Use("test"))

			var errOut bytes.Buffer
			cli := NewCli(nil, nil, &errOut, WithConfig(cfg))

			assert.Equal(t, c.status, cli.Exit(c.err))
			assert.Equal(t, c.said, errOut.String())
		})
	}

	t.Run("not logged in to a context the globals name says which to log in to", func(t *testing.T) {
		t.Parallel()

		cfg := configIn(t, config.Context{Name: "other", API: "https://other.example"})

		var errOut bytes.Buffer
		cli := NewCli(nil, nil, &errOut, WithConfig(cfg))
		cli.Globals.Context = "other"

		assert.Equal(t, console.ExitFailure, cli.Exit(auth.ErrNotLoggedIn))
		assert.Equal(t, "Error: not logged in to \"other\" (https://other.example): run 'tarhche --context other login' first\n", errOut.String())
	})

	t.Run("not logged in to a context that does not exist says so alone", func(t *testing.T) {
		t.Parallel()

		var errOut bytes.Buffer
		cli := NewCli(nil, nil, &errOut, WithConfig(configIn(t)))
		cli.Globals.Context = "missing"

		assert.Equal(t, console.ExitFailure, cli.Exit(auth.ErrNotLoggedIn))
		assert.Equal(t, "Error: not logged in\n", errOut.String())
	})
}

func TestStatusError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "exit status 3", StatusError{Code: 3}.Error())
	assert.Equal(t, "failed", StatusError{Code: 3, Message: "failed"}.Error())
}

func TestArgs(t *testing.T) {
	t.Parallel()

	flags := func(args ...string) *console.FlagSet {
		f := console.NewFlagSet("tarhche rm", nil)
		require.NoError(t, f.Parse(args))

		return f
	}

	assert.NoError(t, NoArgs(flags()))
	assert.EqualError(t, NoArgs(flags("a")), `"tarhche rm" accepts no arguments.`)

	assert.NoError(t, ExactArgs(flags("a"), 1))
	assert.EqualError(t, ExactArgs(flags(), 1), `"tarhche rm" requires exactly 1 argument.`)
	assert.EqualError(t, ExactArgs(flags("a"), 2), `"tarhche rm" requires exactly 2 arguments.`)

	assert.NoError(t, MinArgs(flags("a", "b"), 1))
	assert.EqualError(t, MinArgs(flags(), 1), `"tarhche rm" requires at least 1 argument.`)

	assert.NoError(t, MaxArgs(flags("a"), 1))
	assert.EqualError(t, MaxArgs(flags("a", "b"), 1), `"tarhche rm" requires at most 1 argument.`)

	var usage UsageError
	require.ErrorAs(t, MinArgs(flags(), 1), &usage)
	assert.Equal(t, "tarhche rm", usage.Command)
}

type named struct{}

func (named) Name() string                           { return "ls" }
func (named) Description() string                    { return "lists them." }
func (named) Usage() string                          { return "tarhche container ls" }
func (named) Configure(*console.FlagSet)             {}
func (named) Run(context.Context) console.ExitStatus { return 7 }

func TestAlias(t *testing.T) {
	t.Parallel()

	aliased := Alias(named{}, "ps", "tarhche ps")

	assert.Equal(t, "ps", aliased.Name())
	assert.Equal(t, "tarhche ps", aliased.Usage())
	assert.Equal(t, "lists them.", aliased.Description())
	assert.Equal(t, 7, aliased.Run(context.Background()))
}

func TestCliLanguage(t *testing.T) {
	t.Parallel()

	t.Run("english unless told otherwise", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, DefaultLanguage, NewCli(nil, nil, nil).Language())
	})

	t.Run("the one the globals name", func(t *testing.T) {
		t.Parallel()

		cli := NewCli(nil, nil, nil)
		cli.Globals.Language = "fa"

		assert.Equal(t, "fa", cli.Language())
	})
}
