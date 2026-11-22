package system

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

// saved is the configuration of a harness as it was saved.
func saved(t *testing.T, h *commandtest.Harness) *config.File {
	t.Helper()

	cfg, err := config.Load(h.Config.Dir())
	require.NoError(t, err)

	return cfg
}

// withContexts is a harness whose configuration has the contexts "other"
// and "spare" beside "test", the current one.
func withContexts(t *testing.T, store credentials.Store, seen *visits) *commandtest.Harness {
	t.Helper()

	h := newHarness(t, estate(seen, nil), store)
	require.NoError(t, h.Config.SetContext(config.Context{Name: "other", Description: "Another", API: h.Server.URL, Ingress: "https://workload.other.example", OAuthClient: &config.OAuthClient{ID: "client-7", RedirectURI: "http://127.0.0.1:4321/callback"}}))
	require.NoError(t, h.Config.SetContext(config.Context{Name: "spare", API: "https://spare.example"}))
	require.NoError(t, h.Config.Save())

	return h
}

func TestListContextsCommand(t *testing.T) {
	t.Parallel()

	t.Run("lists every context, the one talked to marked", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newListContextsCommand(h.Cli)), h.Err.String())

		rows := strings.Split(strings.TrimSpace(h.Out.String()), "\n")
		require.Len(t, rows, 5)
		assert.Equal(t, []string{"NAME", "DESCRIPTION", "API", "INGRESS"}, strings.Fields(rows[0]))
		assert.Equal(t, []string{"default", "Tarhche", config.DefaultAPI, config.DefaultIngress}, strings.Fields(rows[1]))
		assert.Equal(t, []string{"other", "Another", h.Server.URL, "https://workload.other.example"}, strings.Fields(rows[2]))
		assert.Equal(t, []string{"spare", "https://spare.example"}, strings.Fields(rows[3]))
		assert.Equal(t, []string{"test", "*", h.Server.URL, h.Server.URL}, strings.Fields(rows[4]))
	})

	t.Run("marks the context the global flags name", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)
		h.Cli.Globals.Context = "default"

		require.Equal(t, console.ExitSuccess, h.Run(newListContextsCommand(h.Cli), "--format", "{{.Name}} {{.Current}}"))

		assert.Equal(t, "default true\nother false\nspare false\ntest false\n", h.Out.String())
	})

	t.Run("lists their names alone", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newListContextsCommand(h.Cli), "-q"))

		assert.Equal(t, "default\nother\nspare\ntest\n", h.Out.String())
	})

	t.Run("lists them as json", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newListContextsCommand(h.Cli), "--format", "json"))

		rows := strings.Split(strings.TrimSpace(h.Out.String()), "\n")
		require.Len(t, rows, 4)

		var current contextView
		require.NoError(t, json.Unmarshal([]byte(rows[3]), &current))
		assert.Equal(t, contextView{Name: "test", API: h.Server.URL, Ingress: h.Server.URL, Current: true}, current)
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newListContextsCommand(h.Cli)))
	})

	t.Run("refuses an argument", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newListContextsCommand(h.Cli), "test"))
	})
}

func TestCreateContextCommand(t *testing.T) {
	t.Parallel()

	t.Run("creates a context and keeps it", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newCreateContextCommand(h.Cli), "--api", "https://prod.example", "--ingress", "https://workload.prod.example", "--description", "Production", "prod"), h.Err.String())

		assert.Equal(t, "prod\n", h.Out.String())
		assert.Equal(t, "Successfully created context \"prod\"\n", h.Err.String())

		created, err := saved(t, h).Context("prod")
		require.NoError(t, err)
		assert.Equal(t, config.Context{Name: "prod", Description: "Production", API: "https://prod.example", Ingress: "https://workload.prod.example"}, created)
	})

	for _, name := range []string{"test", config.DefaultContextName} {
		t.Run("fails for "+name+", which exists", func(t *testing.T) {
			t.Parallel()

			h := commandtest.New(t, nil)

			assert.Equal(t, console.ExitFailure, h.Run(newCreateContextCommand(h.Cli), "--api", "https://prod.example", name))
			assert.Equal(t, "Error: context \""+name+"\" already exists\n", h.Err.String())
		})
	}

	t.Run("fails for a context that cannot be kept as it is", func(t *testing.T) {
		t.Parallel()

		for _, args := range [][]string{{"--api", "https://prod.example", "bad/name"}, {"--api", "not a url", "prod"}, {"--api", "https://prod.example", "--ingress", "ftp://x", "prod"}} {
			h := commandtest.New(t, nil)

			assert.Equal(t, console.ExitFailure, h.Run(newCreateContextCommand(h.Cli), args...), args)
			assert.Contains(t, h.Err.String(), "invalid context", args)
		}
	})

	t.Run("fails for a configuration that cannot be saved", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil, command.WithConfig(unsavable(t, "https://backend.example")))

		assert.Equal(t, console.ExitFailure, h.Run(newCreateContextCommand(h.Cli), "--api", "https://prod.example", "prod"))
		assert.Empty(t, h.Out.String())
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateContextCommand(h.Cli), "--api", "https://prod.example", "prod"))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateContextCommand(unwritable(h)), "--api", "https://prod.example", "prod"))
	})

	usages := map[string][]string{
		"no api":           {"prod"},
		"no name":          {"--api", "https://prod.example"},
		"more than a name": {"--api", "https://prod.example", "prod", "staging"},
	}

	for name, args := range usages {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()

			h := commandtest.New(t, nil)

			assert.Equal(t, console.ExitUsageError, h.Run(newCreateContextCommand(h.Cli), args...))
		})
	}
}

func TestUpdateContextCommand(t *testing.T) {
	t.Parallel()

	t.Run("changes what it is given, and keeps the rest", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(map[string]credentials.Credentials{"other": signedIn(credentials.MethodOAuth)})
		h := withContexts(t, store, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newUpdateContextCommand(h.Cli), "--description", "", "--ingress", "https://workload.example", "--api", h.Server.URL, "other"), h.Err.String())

		assert.Equal(t, "other\n", h.Out.String())
		assert.Equal(t, "Successfully updated context \"other\"\n", h.Err.String())

		updated, err := saved(t, h).Context("other")
		require.NoError(t, err)
		assert.Equal(t, config.Context{Name: "other", API: h.Server.URL, Ingress: "https://workload.example", OAuthClient: &config.OAuthClient{ID: "client-7", RedirectURI: "http://127.0.0.1:4321/callback"}}, updated)

		_, err = store.Get("other")
		assert.NoError(t, err, "the session stays")
	})

	t.Run("moved to another api, logs out of the estate it was at and forgets its application there", func(t *testing.T) {
		t.Parallel()

		seen := &visits{}
		store := mock.NewStore(map[string]credentials.Credentials{"other": signedIn(credentials.MethodOAuth)})
		h := withContexts(t, store, seen)

		require.Equal(t, console.ExitSuccess, h.Run(newUpdateContextCommand(h.Cli), "--api", "https://moved.example", "other"), h.Err.String())

		updated, err := saved(t, h).Context("other")
		require.NoError(t, err)
		assert.Equal(t, config.Context{Name: "other", Description: "Another", API: "https://moved.example", Ingress: "https://workload.other.example"}, updated)

		_, err = store.Get("other")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
		assert.Equal(t, []string{"DELETE /api/dashboard/profile/sessions/session-9"}, seen.asked())
	})

	t.Run("changes the default context too", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newUpdateContextCommand(h.Cli), "--description", "Mine", config.DefaultContextName), h.Err.String())

		updated, err := saved(t, h).Context(config.DefaultContextName)
		require.NoError(t, err)
		assert.Equal(t, "Mine", updated.Description)
		assert.Equal(t, config.DefaultAPI, updated.API)
	})

	t.Run("fails for a context that cannot be kept as it is, and changes nothing", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(map[string]credentials.Credentials{"other": signedIn(credentials.MethodOAuth)})
		h := withContexts(t, store, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "--api", "not a url", "other"))

		unchanged, err := saved(t, h).Context("other")
		require.NoError(t, err)
		assert.Equal(t, h.Server.URL, unchanged.API)

		_, err = store.Get("other")
		assert.NoError(t, err)
	})

	t.Run("fails for a session that cannot be forgotten, and changes nothing", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(operation, _ string) error {
			if operation == "delete" {
				return errors.New("keychain locked")
			}

			return nil
		}

		h := withContexts(t, store, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "--api", "https://moved.example", "other"))

		unchanged, err := h.Cli.Config()
		require.NoError(t, err)

		other, err := unchanged.Context("other")
		require.NoError(t, err)
		assert.Equal(t, h.Server.URL, other.API)
	})

	t.Run("fails for sessions that cannot be kept anywhere", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.CredentialsStore = "vault"

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "--api", "https://moved.example", "test"))
	})

	t.Run("fails for a configuration that cannot be saved", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil, command.WithConfig(unsavable(t, "https://backend.example")))

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "--description", "x", "test"))
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "--description", "x", "missing"))
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(h.Cli), "test"))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUpdateContextCommand(unwritable(h)), "--description", "x", "test"))
	})

	t.Run("refuses no name", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newUpdateContextCommand(h.Cli), "--description", "x"))
	})
}

func TestUseContextCommand(t *testing.T) {
	t.Parallel()

	t.Run("makes a context the current one, and keeps it so", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newUseContextCommand(h.Cli), "other"), h.Err.String())

		assert.Equal(t, "other\n", h.Out.String())
		assert.Equal(t, "Current context is now \"other\"\n", h.Err.String())
		assert.Equal(t, "other", saved(t, h).Current())
	})

	t.Run("makes the default context the current one", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newUseContextCommand(h.Cli), config.DefaultContextName))

		assert.Equal(t, config.DefaultContextName, saved(t, h).Current())
		assert.Empty(t, saved(t, h).CurrentContext)
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUseContextCommand(h.Cli), "missing"))
		assert.Equal(t, "Error: no such context: \"missing\"\n", h.Err.String())
	})

	t.Run("fails for a configuration that cannot be saved", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil, command.WithConfig(unsavable(t, "https://backend.example")))

		assert.Equal(t, console.ExitFailure, h.Run(newUseContextCommand(h.Cli), "test"))
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newUseContextCommand(h.Cli), "test"))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newUseContextCommand(unwritable(h)), "test"))
	})

	t.Run("refuses no name", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newUseContextCommand(h.Cli)))
	})
}

func TestRemoveContextsCommand(t *testing.T) {
	t.Parallel()

	t.Run("removes contexts, logging out of them", func(t *testing.T) {
		t.Parallel()

		seen := &visits{}
		store := mock.NewStore(map[string]credentials.Credentials{"other": signedIn(credentials.MethodOAuth), "test": signedIn(credentials.MethodOAuth)})
		h := withContexts(t, store, seen)

		require.Equal(t, console.ExitSuccess, h.Run(newRemoveContextsCommand(h.Cli), "other", "spare"), h.Err.String())

		assert.Equal(t, "other\nspare\n", h.Out.String())
		assert.Equal(t, []string{config.DefaultContextName, "test"}, saved(t, h).Names())

		_, err := store.Get("other")
		assert.ErrorIs(t, err, credentials.ErrNotFound)

		_, err = store.Get("test")
		assert.NoError(t, err)
		assert.Equal(t, []string{"DELETE /api/dashboard/profile/sessions/session-9"}, seen.asked())
	})

	t.Run("refuses to remove the context in use unless it is forced", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "test"))
		assert.Equal(t, "Error: context \"test\" is in use, set -f flag to force remove\n", h.Err.String())
		assert.Contains(t, saved(t, h).Names(), "test")
	})

	t.Run("removes the context in use when it is forced, and talks to the default one", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newRemoveContextsCommand(h.Cli), "-f", "test"), h.Err.String())

		assert.Equal(t, "test\n", h.Out.String())
		assert.NotContains(t, saved(t, h).Names(), "test")
		assert.Equal(t, config.DefaultContextName, saved(t, h).Current())
	})

	t.Run("forced, forgets the session of a context that is not there", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(map[string]credentials.Credentials{"gone": signedIn(credentials.MethodOAuth)})
		h := withContexts(t, store, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newRemoveContextsCommand(h.Cli), "--force", "gone"), h.Err.String())

		assert.Equal(t, "gone\n", h.Out.String())

		_, err := store.Get("gone")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("removes what it can, and says what it could not", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "other", config.DefaultContextName, "missing", "spare"))

		assert.Equal(t, "other\nspare\n", h.Out.String())
		assert.Equal(t, "Error: context \"default\" cannot be removed\nno such context: \"missing\"\n", h.Err.String())
		assert.Equal(t, []string{config.DefaultContextName, "test"}, saved(t, h).Names())
	})

	t.Run("keeps a context it cannot log out of", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(operation, _ string) error {
			if operation == "delete" {
				return errors.New("keychain locked")
			}

			return nil
		}

		h := withContexts(t, store, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "other"))
		assert.Equal(t, "Error: keychain locked\n", h.Err.String())
		assert.Contains(t, saved(t, h).Names(), "other")
	})

	t.Run("fails for a configuration that cannot be saved", func(t *testing.T) {
		t.Parallel()

		cfg := unsavable(t, "https://backend.example")
		require.NoError(t, cfg.SetContext(config.Context{Name: "other", API: "https://backend.example"}))

		h := commandtest.New(t, nil, command.WithConfig(cfg), command.WithAuth(auth.New(cfg, emptyStore())))

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "other"))
		assert.Empty(t, h.Out.String())
	})

	t.Run("fails for sessions that cannot be kept anywhere", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.CredentialsStore = "vault"

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "test"))
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(h.Cli), "test"))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveContextsCommand(unwritable(h)), "spare"))
	})

	t.Run("refuses no name", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newRemoveContextsCommand(h.Cli), "-f"))
	})
}

func TestInspectContextsCommand(t *testing.T) {
	t.Parallel()

	t.Run("inspects the current context as json", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newInspectContextsCommand(h.Cli)), h.Err.String())

		var inspected []inspectedContext
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &inspected))
		assert.Equal(t, []inspectedContext{{Name: "test", API: h.Server.URL, Ingress: h.Server.URL, Current: true}}, inspected)
		assert.True(t, strings.HasPrefix(h.Out.String(), "[\n    {\n"), "indented as docker indents it")
	})

	t.Run("inspects the contexts named, with their applications", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newInspectContextsCommand(h.Cli), "other", config.DefaultContextName), h.Err.String())

		var inspected []inspectedContext
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &inspected))
		assert.Equal(t, []inspectedContext{
			{
				Name: "other", Description: "Another", API: h.Server.URL, Ingress: "https://workload.other.example",
				OAuthClient: &oauthClientView{ID: "client-7", RedirectURI: "http://127.0.0.1:4321/callback"},
			},
			{Name: config.DefaultContextName, Description: "Tarhche", API: config.DefaultAPI, Ingress: config.DefaultIngress},
		}, inspected)
	})

	t.Run("inspects them through a template", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		require.Equal(t, console.ExitSuccess, h.Run(newInspectContextsCommand(h.Cli), "-f", "{{.Name}} {{.OAuthClient.ID}}", "other"))

		assert.Equal(t, "other client-7\n", h.Out.String())
	})

	t.Run("inspects what it can, and says what it could not", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContextsCommand(h.Cli), "--format", "json", "missing", "spare"))

		assert.Equal(t, `{"Name":"spare","Description":"","API":"https://spare.example","Ingress":"","Current":false}`+"\n", h.Out.String())
		assert.Equal(t, "Error: no such context: \"missing\"\n", h.Err.String())
	})

	t.Run("inspects nothing when nothing named is there", func(t *testing.T) {
		t.Parallel()

		h := withContexts(t, emptyStore(), nil)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContextsCommand(h.Cli), "missing"))

		assert.Equal(t, "[]\n", h.Out.String())
	})

	t.Run("fails for a template that is none", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContextsCommand(h.Cli), "--format", "{{"))
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectContextsCommand(h.Cli)))
	})
}

func TestShowContextCommand(t *testing.T) {
	t.Parallel()

	t.Run("prints the name of the current context", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		require.Equal(t, console.ExitSuccess, h.Run(newShowContextCommand(h.Cli)))

		assert.Equal(t, "test\n", h.Out.String())
	})

	t.Run("prints the name of the context the global flags name", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli.Globals.Context = "other"

		require.Equal(t, console.ExitSuccess, h.Run(newShowContextCommand(h.Cli)))

		assert.Equal(t, "other\n", h.Out.String())
	})

	t.Run("fails for a configuration that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Cli = unreadable(t, h)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContextCommand(h.Cli)))
	})

	t.Run("fails when its output cannot be written", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitFailure, h.Run(newShowContextCommand(unwritable(h))))
	})

	t.Run("refuses an argument", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newShowContextCommand(h.Cli), "test"))
	})
}
