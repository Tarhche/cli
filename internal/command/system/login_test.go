package system

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"testing/iotest"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

func TestLoginCommand(t *testing.T) {
	t.Parallel()

	t.Run("logs in to the current context in a browser", func(t *testing.T) {
		t.Parallel()

		store := emptyStore()
		h := newHarness(t, estate(nil, nil), store)

		require.Equal(t, console.ExitSuccess, h.Run(newLoginCommand(h.Cli)), h.Err.String())

		assert.Equal(t, "Login Succeeded\n", h.Out.String())
		assert.Contains(t, h.Err.String(), `Opening a browser to log in to "test" (`+h.Server.URL+`)`)

		kept, err := store.Get("test")
		require.NoError(t, err)
		assert.Equal(t, credentials.MethodOAuth, kept.Method)
		assert.Equal(t, "client-1", kept.ClientID)
	})

	t.Run("logs in to the context the global flags name", func(t *testing.T) {
		t.Parallel()

		store := emptyStore()
		h := newHarness(t, estate(nil, nil), store)
		require.NoError(t, h.Config.SetContext(config.Context{Name: "other", API: h.Server.URL}))
		h.Cli.Globals.Context = "other"

		require.Equal(t, console.ExitSuccess, h.Run(newLoginCommand(h.Cli)), h.Err.String())

		_, err := store.Get("other")
		require.NoError(t, err)

		_, err = store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("logs in with a password taken from stdin", func(t *testing.T) {
		t.Parallel()

		store := emptyStore()
		h := newHarness(t, estate(nil, nil), store)
		h.In.Reset("secret\r\n")

		require.Equal(t, console.ExitSuccess, h.Run(newLoginCommand(h.Cli), "-u", "jane", "--password-stdin"), h.Err.String())

		assert.Equal(t, "Login Succeeded\n", h.Out.String())
		assert.Empty(t, h.Err.String())

		kept, err := store.Get("test")
		require.NoError(t, err)
		assert.Equal(t, credentials.MethodPassword, kept.Method)
	})

	t.Run("logs in with a password given, warning that it is insecure", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())

		require.Equal(t, console.ExitSuccess, h.Run(newLoginCommand(h.Cli), "--username", "jane", "--password", "secret"), h.Err.String())

		assert.Equal(t, "WARNING! Using --password via the CLI is insecure. Use --password-stdin.\n", h.Err.String())
	})

	t.Run("asks for the password at a terminal", func(t *testing.T) {
		t.Parallel()

		store := emptyStore()
		h := newHarness(t, estate(nil, nil), store)

		login := newLoginCommand(h.Cli)
		login.interactive = func() bool { return true }
		login.readPassword = func() (string, error) { return "secret", nil }

		require.Equal(t, console.ExitSuccess, h.Run(login, "-u", "jane"), h.Err.String())

		assert.Equal(t, "Password: \n", h.Err.String())

		_, err := store.Get("test")
		assert.NoError(t, err)
	})

	passwords := []struct {
		name  string
		setup func(*commandtest.Harness, *loginCommand) []string
		said  string
	}{
		{
			name: "a password nobody typed",
			setup: func(_ *commandtest.Harness, login *loginCommand) []string {
				login.interactive = func() bool { return true }
				login.readPassword = func() (string, error) { return "", nil }

				return []string{"-u", "jane"}
			},
			said: "Error: password required\n",
		},
		{
			name: "a password that cannot be read",
			setup: func(_ *commandtest.Harness, login *loginCommand) []string {
				login.interactive = func() bool { return true }
				login.readPassword = func() (string, error) { return "", errors.New("interrupted") }

				return []string{"-u", "jane"}
			},
			said: "Error: reading the password: interrupted\n",
		},
		{
			name: "an empty password on stdin",
			setup: func(h *commandtest.Harness, _ *loginCommand) []string {
				h.In.Reset("\n")

				return []string{"-u", "jane", "--password-stdin"}
			},
			said: "Error: password required\n",
		},
		{
			name: "nobody at a terminal to type one",
			setup: func(*commandtest.Harness, *loginCommand) []string {
				return []string{"-u", "jane"}
			},
			said: "Error: cannot perform an interactive login from a non TTY device\n",
		},
		{
			name: "a password the estate refuses",
			setup: func(h *commandtest.Harness, _ *loginCommand) []string {
				h.In.Reset("wrong")

				return []string{"-u", "jane", "--password-stdin"}
			},
			said: "Error: login refused: identity: invalid identity or password\n",
		},
	}

	for _, p := range passwords {
		t.Run("fails for "+p.name, func(t *testing.T) {
			t.Parallel()

			store := emptyStore()
			h := newHarness(t, estate(nil, nil), store)
			login := newLoginCommand(h.Cli)

			args := p.setup(h, login)

			assert.Equal(t, console.ExitFailure, h.Run(login, args...))
			assert.Contains(t, h.Err.String(), p.said)
			assert.Empty(t, h.Out.String())

			_, err := store.Get("test")
			assert.ErrorIs(t, err, credentials.ErrNotFound)
		})
	}

	usages := map[string][]string{
		"a password without a username":             {"-p", "secret"},
		"a password on stdin without a username":    {"--password-stdin"},
		"a password given and one on stdin at once": {"-u", "jane", "-p", "secret", "--password-stdin"},
		"an argument": {"default"},
	}

	for name, args := range usages {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t, estate(nil, nil), emptyStore())

			assert.Equal(t, console.ExitUsageError, h.Run(newLoginCommand(h.Cli), args...))
			assert.Contains(t, h.Err.String(), "See 'tarhche login --help'.")
		})
	}

	t.Run("fails for a password on stdin that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())

		manager, err := h.Cli.Auth()
		require.NoError(t, err)

		h.Cli = command.NewCli(iotest.ErrReader(errors.New("broken pipe")), h.Out, h.Err, command.WithConfig(h.Config), command.WithAuth(manager))

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli), "-u", "jane", "--password-stdin"))
		assert.Equal(t, "Error: reading the password from stdin: broken pipe\n", h.Err.String())
	})

	t.Run("fails for a code the estate will not exchange", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"POST /oauth/token": func(rw http.ResponseWriter, _ *http.Request) {
				answer(rw, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"the code is not valid"}`)
			},
		}), emptyStore())

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), "Error: collecting the session: invalid_grant: the code is not valid\n")
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())
		h.Cli.Globals.Context = "missing"

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), `no such context: "missing"`)
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli)))
	})

	t.Run("fails for sessions that cannot be kept anywhere", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, estate(nil, nil))
		h.Config.CredentialsStore = "vault"

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), `the credentials store "vault" is neither "keychain" nor "file"`)
	})

	t.Run("reads no password from an input that is no file, which is no terminal", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		_, err := newLoginCommand(h.Cli).readTerminalPassword()

		assert.EqualError(t, err, "the input is not a terminal")
	})

	t.Run("reads no password from a file that is no terminal", func(t *testing.T) {
		t.Parallel()

		file, err := os.CreateTemp(t.TempDir(), "input")
		require.NoError(t, err)
		t.Cleanup(func() { _ = file.Close() })

		_, err = newLoginCommand(command.NewCli(file, nil, nil)).readTerminalPassword()

		assert.Error(t, err)
	})

	t.Run("fails for a session that cannot be kept", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(string, string) error { return errors.New("keychain locked") }

		h := newHarness(t, estate(nil, nil), store)

		assert.Equal(t, console.ExitFailure, h.Run(newLoginCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), "Error: keychain locked\n")
	})
}
