package content

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

// sessionStore keeps sessions in a map, refusing to keep any when told to.
type sessionStore struct {
	lock    sync.Mutex
	kept    map[string]credentials.Credentials
	refuse  error
	deleted []string
}

var _ credentials.Store = &sessionStore{}

func (s *sessionStore) Get(name string) (credentials.Credentials, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	kept, found := s.kept[name]
	if !found {
		return credentials.Credentials{}, credentials.ErrNotFound
	}

	return kept, nil
}

func (s *sessionStore) Set(name string, session credentials.Credentials) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.refuse != nil {
		return s.refuse
	}

	if s.kept == nil {
		s.kept = make(map[string]credentials.Credentials)
	}

	s.kept[name] = session

	return nil
}

func (s *sessionStore) Delete(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.kept, name)
	s.deleted = append(s.deleted, name)

	return nil
}

func TestImpersonateCommand(t *testing.T) {
	t.Parallel()

	const opened = `{"access_token":"at","refresh_token":"rt","user":{"uuid":"u1","name":"Jane","username":"jane"}}`

	shadows := func(answer string) *fakeAPI {
		return newFakeAPI(map[string][]reply{"POST /api/dashboard/users/u1/impersonate": {{status: http.StatusOK, body: answer}}})
	}

	impersonate := func(h *commandtest.Harness, store *sessionStore) *impersonateCommand {
		c := newImpersonateCommand(h.Cli, "tarhche user")
		c.open = func(*config.File) (credentials.Store, error) { return store, nil }

		return c
	}

	saved := func(t *testing.T, h *commandtest.Harness) *config.File {
		t.Helper()

		cfg, err := config.Load(h.Config.Dir())
		require.NoError(t, err)

		return cfg
	}

	t.Run("a shadow session kept as a context beside the one it was opened from", func(t *testing.T) {
		t.Parallel()

		api := shadows(opened)
		h := harness(t, api)
		store := &sessionStore{}

		status := h.Run(impersonate(h, store), "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"POST /api/dashboard/users/u1/impersonate"}, api.routes())
		assert.Empty(t, api.requests()[0].Body)
		assert.Equal(t, "test-as-jane\n", h.Out.String())
		assert.Equal(t, credentials.Credentials{Method: credentials.MethodPassword, AccessToken: "at", RefreshToken: "rt"}, store.kept["test-as-jane"])

		shadow, err := saved(t, h).Context("test-as-jane")
		require.NoError(t, err)
		assert.Equal(t, config.Context{Name: "test-as-jane", Description: "jane, impersonated from test", API: h.Server.URL, Ingress: h.Server.URL}, shadow)
		assert.Equal(t, "test", saved(t, h).Current(), "the context in use stays the one it was")
	})

	t.Run("kept as the context named", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(opened))
		store := &sessionStore{}

		status := h.Run(impersonate(h, store), "--name", "as-jane", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "as-jane\n", h.Out.String())
		assert.Contains(t, store.kept, "as-jane")
	})

	t.Run("named after the uuid of a user without a username", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(`{"access_token":"at","refresh_token":"rt","user":{"uuid":"u1"}}`))

		status := h.Run(impersonate(h, &sessionStore{}), "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "test-as-u1\n", h.Out.String())
	})

	t.Run("a name a context may not have", func(t *testing.T) {
		t.Parallel()

		api := shadows(opened)
		h := harness(t, api)

		status := h.Run(impersonate(h, &sessionStore{}), "--name", "-bad", "u1")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "is not a valid name")
		assert.Empty(t, api.routes())
	})

	t.Run("no store to keep it in opens no session", func(t *testing.T) {
		t.Parallel()

		api := shadows(opened)
		h := harness(t, api)

		c := newImpersonateCommand(h.Cli, "tarhche user")
		c.open = func(*config.File) (credentials.Store, error) { return nil, errors.New("no keychain") }

		status := h.Run(c, "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no keychain\n", h.Err.String())
		assert.Empty(t, api.routes())
	})

	t.Run("a user that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(nil))
		store := &sessionStore{}

		status := h.Run(impersonate(h, store), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such user: u1\n", h.Err.String())
		assert.Empty(t, store.kept)
	})

	t.Run("an answer that opens no session", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(`{"errors":{}}`))
		store := &sessionStore{}

		status := h.Run(impersonate(h, store), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: the api opened no session\n", h.Err.String())
		assert.Empty(t, store.kept)
	})

	t.Run("a session that cannot be kept keeps no context", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(opened))

		status := h.Run(impersonate(h, &sessionStore{refuse: errors.New("locked")}), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: locked\n", h.Err.String())

		_, err := saved(t, h).Context("test-as-jane")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("a context that cannot be kept keeps no session", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(opened))
		store := &sessionStore{}

		require.NoError(t, os.RemoveAll(h.Config.Dir()))
		require.NoError(t, os.WriteFile(h.Config.Dir(), nil, 0o600))

		status := h.Run(impersonate(h, store), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Empty(t, store.kept)
		assert.Equal(t, []string{"test-as-jane"}, store.deleted)
	})

	for name, args := range map[string][]string{
		"no uuid":          nil,
		"two uuids":        {"u1", "u2"},
		"a uuid it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := shadows(opened)
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(impersonate(h, &sessionStore{}), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		c := newImpersonateCommand(nil, "tarhche user")

		assert.Equal(t, "impersonate", c.Name())
		assert.Equal(t, "Open a shadow session as a user, kept as a context of its own", c.Description())
		assert.Equal(t, "tarhche user impersonate [OPTIONS] UUID", c.Usage())
	})

	t.Run("without a context to open it from", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newImpersonateCommand(cli, "tarhche user") }, "u1")
	})

	t.Run("a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{`), 0o600))

		out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
		cli := command.NewCli(strings.NewReader(""), out, errOut)
		cli.Globals.ConfigDir = dir

		status := runOn(newImpersonateCommand(cli, "tarhche user"), out, errOut, "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, errOut.String(), "reading ")
	})

	t.Run("an api that cannot be reached", func(t *testing.T) {
		t.Parallel()

		h := harness(t, shadows(opened))

		out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
		cli := command.NewCli(strings.NewReader(""), out, errOut, command.WithConfig(h.Config), command.WithTokens(commandtest.Tokens(commandtest.Token)))
		cli.Globals.Host = "nowhere"

		c := newImpersonateCommand(cli, "tarhche user")
		c.open = func(*config.File) (credentials.Store, error) { return &sessionStore{}, nil }

		status := runOn(c, out, errOut, "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, errOut.String(), `"nowhere" is not an absolute http or https url`)
	})

	t.Run("a context that may not be kept", func(t *testing.T) {
		t.Parallel()

		store := &sessionStore{}
		err := newImpersonateCommand(nil, "tarhche user").keep(harness(t, nil).Config, store, config.Context{Name: "-bad", API: "https://x.example"}, credentials.Credentials{})

		assert.ErrorIs(t, err, config.ErrInvalidContext)
		assert.Empty(t, store.kept)
	})
}

func TestContextNameOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "default-as-jane.doe_1+x", contextNameOf("default-as-jane.doe_1+x"))
	assert.Equal(t, "default-as-a-b-c", contextNameOf("default-as-a b/c"))
}
