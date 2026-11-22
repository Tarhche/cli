package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

// lines are the lines of a default output, each label padded as its
// columns are.
func lines(rows ...[2]string) string {
	var text strings.Builder

	for _, row := range rows {
		if len(row[0]) == 0 {
			text.WriteString("\n")

			continue
		}

		fmt.Fprintf(&text, "%-20s%s\n", row[0], row[1])
	}

	return text.String()
}

func answering(body string) http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) { answer(rw, http.StatusOK, body) }
}

func TestInfoCommand(t *testing.T) {
	t.Parallel()

	loggedIn := func(method string) *mock.Store {
		return mock.NewStore(map[string]credentials.Credentials{"test": signedIn(method)})
	}

	t.Run("shows the context, who it is logged in as, their roles and their subscription", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		assert.Equal(t, lines(
			[2]string{"Context:", "test"},
			[2]string{" API:", h.Server.URL},
			[2]string{" Ingress:", h.Server.URL},
			[2]string{" Logged in:", "yes, in a browser"},
			[2]string{},
			[2]string{"User:", "Jane Doe"},
			[2]string{" Username:", "jane"},
			[2]string{" Email:", "jane@example.com"},
			[2]string{" UUID:", "user-1"},
			[2]string{" Roles:", "Administrator, Writer"},
			[2]string{},
			[2]string{"Subscription:", "Pro"},
			[2]string{" VMs:", "2 of 5"},
			[2]string{" Snapshots:", "1 of 10"},
			[2]string{" Files:", "3"},
		), h.Out.String())
		assert.Empty(t, h.Err.String())
	})

	t.Run("shows a session logged in to with a password, seen as somebody else, past its limits", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile":              answering(`{"uuid":"user-1","name":"Jane Doe","email":"jane@example.com","username":"jane","impersonated_by":{"uuid":"admin-1","name":"Admin","username":"admin"}}`),
			"GET /api/dashboard/profile/roles":        answering(`{"items":[]}`),
			"GET /api/dashboard/profile/subscription": answering(`{"subscription":{"uuid":"plan-1","names":{},"quotas":{"vms":null,"snapshots":null,"uploads":{"max_size":10,"max":7,"enforcement":{"mode":"hard"}}}},"usage":{"vms":4,"snapshots":2,"files":9},"over":[{"kind":"file"},{"kind":"file"}]}`),
		}), loggedIn(credentials.MethodPassword))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		out := h.Out.String()
		assert.Contains(t, out, lines([2]string{" Logged in:", "yes, with a password"}))
		assert.Contains(t, out, lines([2]string{" Seen by:", "Admin (admin)"}, [2]string{" Roles:", "none"}))
		assert.Contains(t, out, lines(
			[2]string{"Subscription:", "plan-1"},
			[2]string{" VMs:", "4"},
			[2]string{" Snapshots:", "2"},
			[2]string{" Files:", "9 of 7"},
			[2]string{" Past its limits:", "2"},
		))
	})

	t.Run("shows somebody on no subscription", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile/subscription": answering(`{"subscription":null,"usage":{"vms":1,"snapshots":0,"files":2},"over":[]}`),
		}), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		assert.Contains(t, h.Out.String(), lines([2]string{"Subscription:", "none"}, [2]string{" VMs:", "1"}, [2]string{" Snapshots:", "0"}, [2]string{" Files:", "2"}))
	})

	t.Run("names roles and subscriptions in the language asked for", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), loggedIn(credentials.MethodOAuth))
		h.Cli.Globals.Language = "fa"

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		assert.Contains(t, h.Out.String(), lines([2]string{" Roles:", "مدیر, Writer"}))
		assert.Contains(t, h.Out.String(), "ویژه")
	})

	t.Run("names roles in the user's own language, else in any, else by their uuid", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile":       answering(`{"uuid":"user-1","name":"Jane","username":"jane","language_code":"fa"}`),
			"GET /api/dashboard/profile/roles": answering(`{"items":[{"uuid":"role-1","translations":{"en":{"title":"Administrator"},"fa":{"title":"مدیر"}}},{"uuid":"role-2","translations":{"de":{"title":""},"ar":{"title":"كاتب"}}},{"uuid":"role-3","translations":{}}]}`),
		}), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		assert.Contains(t, h.Out.String(), lines([2]string{" Roles:", "مدیر, كاتب, role-3"}))
	})

	t.Run("shows nobody when nobody is logged in, without asking the estate", func(t *testing.T) {
		t.Parallel()

		seen := &visits{}
		h := newHarness(t, estate(seen, nil), emptyStore())

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)), h.Err.String())

		assert.Equal(t, lines(
			[2]string{"Context:", "test"},
			[2]string{" API:", h.Server.URL},
			[2]string{" Ingress:", h.Server.URL},
			[2]string{" Logged in:", "no"},
		), h.Out.String())
		assert.Empty(t, seen.asked())
	})

	t.Run("warns of a subscription that cannot be read, and shows the rest", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile/subscription": func(rw http.ResponseWriter, _ *http.Request) {
				answer(rw, http.StatusInternalServerError, "")
			},
		}), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli)))

		assert.Contains(t, h.Out.String(), "Jane Doe")
		assert.NotContains(t, h.Out.String(), "Subscription:")
		assert.Equal(t, "WARNING: the subscription could not be read: internal server error\n", h.Err.String())
	})

	t.Run("shows it all as json", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile/subscription": func(rw http.ResponseWriter, _ *http.Request) {
				answer(rw, http.StatusBadGateway, "")
			},
		}), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli), "--format", "json"))

		var shown information
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &shown))
		assert.Equal(t, information{
			Context:  contextInfo{Name: "test", API: h.Server.URL, Ingress: h.Server.URL},
			LoggedIn: true,
			Method:   credentials.MethodOAuth,
			User:     &userInfo{UUID: "user-1", Name: "Jane Doe", Username: "jane", Email: "jane@example.com"},
			Roles:    []roleInfo{{UUID: "role-1", Title: "Administrator"}, {UUID: "role-2", Title: "Writer"}},
			Warnings: []string{"the subscription could not be read: bad gateway"},
		}, shown)
		assert.Empty(t, h.Err.String(), "a warning is in the json, not beside it")
	})

	t.Run("shows it through a template", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), loggedIn(credentials.MethodOAuth))

		require.Equal(t, console.ExitSuccess, h.Run(newInfoCommand(h.Cli), "-f", "{{.User.Username}} {{.Subscription.VMs.Used}}"))

		assert.Equal(t, "jane 2\n", h.Out.String())
	})

	for _, route := range []string{"GET /api/dashboard/profile", "GET /api/dashboard/profile/roles"} {
		t.Run("fails when "+route+" fails", func(t *testing.T) {
			t.Parallel()

			h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
				route: func(rw http.ResponseWriter, _ *http.Request) { answer(rw, http.StatusInternalServerError, "") },
			}), loggedIn(credentials.MethodOAuth))

			assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli)))
			assert.Empty(t, h.Out.String())
		})
	}

	t.Run("stops when it is given up on", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		h := newHarness(t, estate(nil, map[string]http.HandlerFunc{
			"GET /api/dashboard/profile/subscription": func(_ http.ResponseWriter, r *http.Request) {
				cancel()
				<-r.Context().Done()
			},
		}), loggedIn(credentials.MethodOAuth))

		assert.Equal(t, command.ExitInterrupted, h.RunContext(ctx, newInfoCommand(h.Cli)))
	})

	t.Run("fails for a session that cannot be read", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(string, string) error { return errors.New("keychain locked") }

		h := newHarness(t, estate(nil, nil), store)

		assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), "Error: keychain locked\n")
	})

	t.Run("fails for an api that is no url", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.Contexts["broken"] = config.Context{Name: "broken", API: "not a url"}

		store := mock.NewStore(map[string]credentials.Credentials{"broken": signedIn(credentials.MethodOAuth)})
		h.Cli = command.NewCli(h.In, h.Out, h.Err, command.WithConfig(h.Config), command.WithAuth(auth.New(h.Config, store)))
		h.Cli.Globals.Context = "broken"

		assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli)))
		assert.Contains(t, h.Err.String(), `"not a url" is not an absolute http or https url`)
	})

	t.Run("fails for a template that is none", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())

		assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli), "--format", "{{"))
	})

	t.Run("fails for a context that does not exist", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())
		h.Cli.Globals.Context = "missing"

		assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli)))
	})

	t.Run("fails for sessions that cannot be kept anywhere", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		h.Config.CredentialsStore = "vault"

		assert.Equal(t, console.ExitFailure, h.Run(newInfoCommand(h.Cli)))
	})

	t.Run("refuses an argument", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, estate(nil, nil), emptyStore())

		assert.Equal(t, console.ExitUsageError, h.Run(newInfoCommand(h.Cli), "everything"))
	})
}

func TestTranslated(t *testing.T) {
	t.Parallel()

	texts := map[string]string{"en": "Pro", "fa": "ویژه", "de": ""}

	assert.Equal(t, "ویژه", translated(texts, "fa", "en"))
	assert.Equal(t, "Pro", translated(texts, "", "de", "en"))
	assert.Equal(t, "Pro", translated(texts, "ar"), "the first language there is, in alphabetical order, that says anything")
	assert.Empty(t, translated(map[string]string{"en": ""}, "en"))
	assert.Empty(t, translated(nil, "en"))
}
