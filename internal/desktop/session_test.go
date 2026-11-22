package desktop_test

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/desktop/mock"
)

func TestSession(t *testing.T) {
	t.Parallel()

	newSession := func(t *testing.T, handler http.Handler) (*desktop.Session, *desktop.Workspace, *mock.Authenticator, *config.File) {
		t.Helper()

		w, h := newWorkspace(t, handler)
		authenticator := &mock.Authenticator{}
		t.Cleanup(func() { authenticator.AssertExpectations(t) })

		return desktop.NewSession(w, authenticator), w, authenticator, h.Config
	}

	contextOf := func(t *testing.T, w *desktop.Workspace) string {
		t.Helper()

		name, err := w.ContextName()
		require.NoError(t, err)

		return name
	}

	t.Run("the contexts are every context, the one it talks to among them", func(t *testing.T) {
		t.Parallel()

		session, _, _, cfg := newSession(t, nil)
		test, err := cfg.Context(testContext)
		require.NoError(t, err)

		contexts, err := session.Contexts()

		require.NoError(t, err)
		assert.Equal(t, []desktop.ContextInfo{
			{Name: config.DefaultContextName, Description: "Tarhche", API: config.DefaultAPI, Ingress: config.DefaultIngress},
			{Name: testContext, API: test.API, Ingress: test.Ingress, Current: true},
		}, contexts)
	})

	t.Run("the current context is the one it talks to", func(t *testing.T) {
		t.Parallel()

		session, _, _, cfg := newSession(t, nil)
		test, err := cfg.Context(testContext)
		require.NoError(t, err)

		current, err := session.Current()

		require.NoError(t, err)
		assert.Equal(t, desktop.ContextInfo{Name: testContext, API: test.API, Ingress: test.Ingress, Current: true}, current)
	})

	t.Run("a configuration that cannot be read has no contexts", func(t *testing.T) {
		t.Parallel()

		session := desktop.NewSession(brokenWorkspace(t), &mock.Authenticator{})

		_, contextsErr := session.Contexts()
		_, currentErr := session.Current()
		_, statusErr := session.Status(context.Background())

		assert.Error(t, contextsErr)
		assert.Error(t, currentErr)
		assert.Error(t, statusErr)
		assert.Error(t, session.LoginWithBrowser(context.Background()))
	})

	t.Run("using a context talks to it, and keeps it current", func(t *testing.T) {
		t.Parallel()

		session, w, _, cfg := newSession(t, nil)
		require.NoError(t, cfg.SetContext(config.Context{Name: "other", API: "https://other.example"}))

		require.NoError(t, session.Use("other"))

		saved, err := config.Load(cfg.Dir())
		require.NoError(t, err)

		assert.Equal(t, "other", contextOf(t, w))
		assert.Equal(t, "other", saved.Current())
	})

	t.Run("a context that does not exist cannot be used", func(t *testing.T) {
		t.Parallel()

		session, w, _, _ := newSession(t, nil)

		err := session.Use("missing")

		assert.ErrorIs(t, err, config.ErrNoSuchContext)
		assert.Equal(t, testContext, contextOf(t, w))
	})

	t.Run("a context is created and kept, and not talked to", func(t *testing.T) {
		t.Parallel()

		session, w, _, cfg := newSession(t, nil)

		err := session.Create(desktop.NewContext{Name: "local", Description: "mine", API: "http://localhost:8000", Ingress: "http://localhost:8030"})
		require.NoError(t, err)

		saved, err := config.Load(cfg.Dir())
		require.NoError(t, err)

		local, err := saved.Context("local")
		require.NoError(t, err)

		assert.Equal(t, config.Context{Name: "local", Description: "mine", API: "http://localhost:8000", Ingress: "http://localhost:8030"}, local)
		assert.Equal(t, testContext, contextOf(t, w))
	})

	t.Run("a context is refused field by field", func(t *testing.T) {
		t.Parallel()

		session, _, _, cfg := newSession(t, nil)

		err := session.Create(desktop.NewContext{Name: "-bad", API: "nope", Ingress: "ftp://x"})

		var invalid desktop.InvalidError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, []string{"api", "ingress", "name"}, slices.Sorted(maps.Keys(invalid.Fields)))
		assert.Contains(t, invalid.Fields["name"], "is not a valid name")
		assert.Equal(t, `"nope" is not an absolute http or https url`, invalid.Fields["api"])
		assert.Equal(t, `"ftp://x" is not an absolute http or https url`, invalid.Fields["ingress"])
		assert.NoFileExists(t, cfg.Path())
	})

	t.Run("a context that exists is not created again", func(t *testing.T) {
		t.Parallel()

		session, _, _, _ := newSession(t, nil)

		for _, name := range []string{testContext, config.DefaultContextName} {
			err := session.Create(desktop.NewContext{Name: name, API: "https://x.example"})

			var invalid desktop.InvalidError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, map[string]string{"name": `a context called "` + name + `" exists already`}, invalid.Fields)
		}
	})

	t.Run("a context is signed out of and forgotten", func(t *testing.T) {
		t.Parallel()

		session, w, authenticator, cfg := newSession(t, nil)
		require.NoError(t, cfg.SetContext(config.Context{Name: "other", API: "https://other.example"}))
		authenticator.On("Logout", testify.Anything, "other").Return(nil).Once()

		require.NoError(t, session.Remove(context.Background(), "other"))

		saved, err := config.Load(cfg.Dir())
		require.NoError(t, err)

		_, err = saved.Context("other")
		assert.ErrorIs(t, err, config.ErrNoSuchContext)
		assert.Equal(t, testContext, contextOf(t, w))
	})

	t.Run("removing the context it talks to talks to the current one", func(t *testing.T) {
		t.Parallel()

		session, w, authenticator, _ := newSession(t, nil)
		authenticator.On("Logout", testify.Anything, testContext).Return(auth.ErrNotLoggedIn).Once()

		require.NoError(t, session.Remove(context.Background(), testContext))

		assert.Equal(t, config.DefaultContextName, contextOf(t, w))
	})

	t.Run("a context that cannot be signed out of is kept", func(t *testing.T) {
		t.Parallel()

		session, _, authenticator, cfg := newSession(t, nil)
		failure := errors.New("the keychain is locked")
		authenticator.On("Logout", testify.Anything, testContext).Return(failure).Once()

		err := session.Remove(context.Background(), testContext)

		assert.ErrorIs(t, err, failure)

		_, err = cfg.Context(testContext)
		assert.NoError(t, err)
	})

	t.Run("a context removed meanwhile is not removed again", func(t *testing.T) {
		t.Parallel()

		session, _, authenticator, cfg := newSession(t, nil)
		require.NoError(t, cfg.SetContext(config.Context{Name: "other", API: "https://other.example"}))
		authenticator.On("Logout", testify.Anything, "other").Run(func(testify.Arguments) {
			assert.NoError(t, cfg.RemoveContext("other"))
		}).Return(nil).Once()

		err := session.Remove(context.Background(), "other")

		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("the default context and one that does not exist are not removed", func(t *testing.T) {
		t.Parallel()

		session, _, _, _ := newSession(t, nil)

		assert.ErrorIs(t, session.Remove(context.Background(), config.DefaultContextName), config.ErrDefaultContext)
		assert.ErrorIs(t, session.Remove(context.Background(), "missing"), config.ErrNoSuchContext)
	})

	t.Run("a context nobody signed in to has no session", func(t *testing.T) {
		t.Parallel()

		for _, said := range []error{nil, auth.ErrNotLoggedIn} {
			session, _, authenticator, _ := newSession(t, nil)
			authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{}, said).Once()

			status, err := session.Status(context.Background())

			require.NoError(t, err)
			assert.False(t, status.LoggedIn)
			assert.Equal(t, testContext, status.Context.Name)
			assert.Nil(t, status.User)
		}
	})

	t.Run("a session that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		session, _, authenticator, _ := newSession(t, nil)
		failure := errors.New("the keychain is locked")
		authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{}, failure).Once()

		_, err := session.Status(context.Background())

		assert.ErrorIs(t, err, failure)
	})

	t.Run("a session is whoever its profile says", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{"uuid":"u-1","name":"Sarah","username":"sarah","email":"s@example.com","avatar":"a.png"}`)
		session, _, authenticator, _ := newSession(t, api)
		expires := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
		authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{LoggedIn: true, Method: "oauth", Subject: "u-1", ExpiresAt: expires}, nil).Once()

		status, err := session.Status(context.Background())

		require.NoError(t, err)
		assert.True(t, status.LoggedIn)
		assert.Equal(t, "oauth", status.Method)
		assert.Equal(t, "u-1", status.Subject)
		assert.Equal(t, expires, status.ExpiresAt)
		assert.Equal(t, &desktop.User{UUID: "u-1", Name: "Sarah", Username: "sarah", Email: "s@example.com", Avatar: "a.png"}, status.User)
		assert.Equal(t, "/api/dashboard/profile", api.last(t).Path)
	})

	t.Run("a session the API refuses is none", func(t *testing.T) {
		t.Parallel()

		session, _, authenticator, _ := newSession(t, newRecorder(http.StatusUnauthorized, ``))
		authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{LoggedIn: true}, nil).Once()

		status, err := session.Status(context.Background())

		require.NoError(t, err)
		assert.False(t, status.LoggedIn)
	})

	t.Run("a session whose profile cannot be read is still one", func(t *testing.T) {
		t.Parallel()

		session, _, authenticator, _ := newSession(t, newRecorder(http.StatusBadGateway, ``))
		authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{LoggedIn: true, Method: "password"}, nil).Once()

		status, err := session.Status(context.Background())

		require.NoError(t, err)
		assert.True(t, status.LoggedIn)
		assert.Equal(t, "password", status.Method)
		assert.Nil(t, status.User)
	})

	t.Run("a session whose API cannot be reached is still one", func(t *testing.T) {
		t.Parallel()

		h := commandtest.New(t, nil)
		authenticator := &mock.Authenticator{}
		authenticator.On("Status", testify.Anything, testContext).Return(auth.Status{LoggedIn: true}, nil).Once()

		w := desktop.NewWorkspace(testContext, func(contextName string) *command.Cli {
			cli := opener(h)(contextName)
			cli.Globals.Host = "not a url"

			return cli
		})

		status, err := desktop.NewSession(w, authenticator).Status(context.Background())

		require.NoError(t, err)
		assert.True(t, status.LoggedIn)
		assert.Nil(t, status.User)
	})

	t.Run("signing in or out talks to the context afresh", func(t *testing.T) {
		t.Parallel()

		session, w, authenticator, _ := newSession(t, nil)
		authenticator.On("LoginWithBrowser", testify.Anything, testContext).Return(nil).Once()
		authenticator.On("LoginWithPassword", testify.Anything, testContext, "sarah", "secret").Return(nil).Once()
		authenticator.On("Logout", testify.Anything, testContext).Return(nil).Once()

		for _, signIn := range []func(context.Context) error{
			session.LoginWithBrowser,
			func(ctx context.Context) error { return session.LoginWithPassword(ctx, "sarah", "secret") },
			session.Logout,
		} {
			before := w.Cli()

			require.NoError(t, signIn(context.Background()))

			assert.NotSame(t, before, w.Cli())
		}
	})

	t.Run("signing in that fails says why", func(t *testing.T) {
		t.Parallel()

		session, w, authenticator, _ := newSession(t, nil)
		failure := errors.New("the browser was closed")
		authenticator.On("LoginWithBrowser", testify.Anything, testContext).Return(failure).Once()
		before := w.Cli()

		err := session.LoginWithBrowser(context.Background())

		assert.ErrorIs(t, err, failure)
		assert.Same(t, before, w.Cli())
	})
}
