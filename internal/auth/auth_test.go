package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("a manager reaches estates with a client of its own, and opens pages in the browser", func(t *testing.T) {
		t.Parallel()

		m := New(nil, nil)

		require.NotNil(t, m.http)
		assert.Equal(t, api.DefaultTimeout, m.http.Timeout)
		assert.NotNil(t, m.browser)
		assert.Equal(t, loginTimeout, m.loginTimeout)
		assert.False(t, m.interactive)
	})

	t.Run("options configure it", func(t *testing.T) {
		t.Parallel()

		client, prompt := &http.Client{}, &bytes.Buffer{}
		opened := ""

		m := New(nil, nil, WithHTTPClient(client), WithPrompt(prompt), WithInteractive(true), WithBrowser(func(page string) error {
			opened = page

			return nil
		}))

		assert.Same(t, client, m.http)
		assert.Same(t, prompt, m.prompt)
		assert.True(t, m.interactive)
		require.NoError(t, m.browser("https://backend.example"))
		assert.Equal(t, "https://backend.example", opened)
	})

	t.Run("a manager with nowhere to say anything says nothing", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() { New(nil, nil).say("hello %s", "there") })
	})
}

func TestManagerLogout(t *testing.T) {
	t.Parallel()

	t.Run("logging out forgets the session and ends it on the estate with its own token", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})

		require.NoError(t, newHarness(t, e, store).Logout(context.Background(), "test"))

		_, err := store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
		assert.Equal(t, []endedSession{{session: claimsOf(kept.AccessToken).Session, token: kept.AccessToken}}, e.endedSessions())
	})

	t.Run("a token about to run out is renewed to end its session", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := oauthSession(e, time.Second)
		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})

		require.NoError(t, newHarness(t, e, store).Logout(context.Background(), "test"))

		ended := e.endedSessions()
		require.Len(t, ended, 1)
		assert.Equal(t, claimsOf(kept.AccessToken).Session, ended[0].session)
		assert.NotEqual(t, kept.AccessToken, ended[0].token)
		assert.Equal(t, 1, e.called("POST /as/token"))
	})

	t.Run("a token about to run out that cannot be renewed ends its session as it is", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := credentials.Credentials{Method: credentials.MethodPassword, AccessToken: expiring("session-9", time.Second), RefreshToken: "unknown"}

		require.NoError(t, newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept})).Logout(context.Background(), "test"))

		assert.Equal(t, []endedSession{{session: "session-9", token: kept.AccessToken}}, e.endedSessions())
	})

	t.Run("the session the refresh token names is ended when the access token names none", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := credentials.Credentials{Method: credentials.MethodPassword, AccessToken: "opaque", RefreshToken: expiring("session-7", time.Hour)}

		require.NoError(t, newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept})).Logout(context.Background(), "test"))

		assert.Equal(t, []endedSession{{session: "session-7", token: "opaque"}}, e.endedSessions())
	})

	t.Run("a session the estate will not end is forgotten all the same", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		e.set(func(e *estate) {
			e.fail["DELETE /api/dashboard/profile/sessions/"+claimsOf(kept.AccessToken).Session] = failure{status: http.StatusUnauthorized}
		})

		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})

		require.NoError(t, newHarness(t, e, store).Logout(context.Background(), "test"))

		_, err := store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("nothing is asked of the estate without a session to end", func(t *testing.T) {
		t.Parallel()

		for name, kept := range map[string]map[string]credentials.Credentials{
			"no credentials":                  nil,
			"tokens that name no session":     {"test": {Method: credentials.MethodPassword, AccessToken: "opaque", RefreshToken: "opaque"}},
			"credentials of a context gone":   {"ghost": {Method: credentials.MethodPassword, AccessToken: expiring("s", time.Hour)}},
			"credentials of a broken context": {"broken": {Method: credentials.MethodPassword, AccessToken: expiring("s", time.Hour)}},
		} {
			e := newEstate(t)
			h := newHarness(t, e, mock.NewStore(kept))
			h.config.Contexts["broken"] = config.Context{Name: "broken", API: "not a url"}

			for contextName := range kept {
				require.NoError(t, h.Logout(context.Background(), contextName), name)
			}

			require.NoError(t, h.Logout(context.Background(), "test"), name)
			assert.Empty(t, e.endedSessions(), name)
		}
	})

	t.Run("credentials that cannot be read are forgotten", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		store := mock.NewStore(map[string]credentials.Credentials{"test": passwordSession(e, time.Minute)})
		store.Fail = failing("get")

		require.NoError(t, newHarness(t, e, store).Logout(context.Background(), "test"))

		store.Fail = nil
		_, err := store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
		assert.Empty(t, e.endedSessions())
	})

	t.Run("credentials that cannot be forgotten are an error", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		store := mock.NewStore(map[string]credentials.Credentials{"test": passwordSession(e, time.Minute)})
		store.Fail = failing("delete")

		assert.EqualError(t, newHarness(t, e, store).Logout(context.Background(), "test"), "delete failed")
		assert.Empty(t, e.endedSessions())
	})

	t.Run("a logout given up on while the session is renewed stops", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		release, err := h.hold(context.Background(), "test")
		require.NoError(t, err)
		t.Cleanup(release)

		assert.ErrorIs(t, h.Logout(canceled(), "test"), context.Canceled)
	})
}

func TestManagerStatus(t *testing.T) {
	t.Parallel()

	t.Run("a context nobody is logged in to says so", func(t *testing.T) {
		t.Parallel()

		status, err := newHarness(t, newEstate(t), mock.NewStore(nil)).Status(context.Background(), "test")

		require.NoError(t, err)
		assert.Equal(t, Status{}, status)
	})

	t.Run("a session says how it was signed in to, whose it is and when its token runs out", func(t *testing.T) {
		t.Parallel()

		expires := time.Now().Add(time.Minute).Truncate(time.Second)
		kept := credentials.Credentials{Method: credentials.MethodOAuth, AccessToken: token(map[string]any{"sub": "user-1", "exp": expires.Unix()})}

		status, err := newHarness(t, newEstate(t), mock.NewStore(map[string]credentials.Credentials{"test": kept})).Status(context.Background(), "test")

		require.NoError(t, err)
		assert.True(t, status.LoggedIn)
		assert.Equal(t, credentials.MethodOAuth, status.Method)
		assert.Equal(t, "user-1", status.Subject)
		assert.True(t, expires.Equal(status.ExpiresAt))
	})

	t.Run("a store that fails is an error", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(string, string) error { return errors.New("locked") }

		_, err := newHarness(t, newEstate(t), store).Status(context.Background(), "test")

		assert.EqualError(t, err, "locked")
	})
}

func TestFixedToken(t *testing.T) {
	t.Parallel()

	got, err := fixedToken("t").Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "t", got)

	_, err = fixedToken("t").Renew(context.Background(), "t")
	assert.ErrorIs(t, err, errNotRenewed)
}
