package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

// passwordSession is a session kept as a password login would have kept
// it, whose access token runs out in lifetime.
func passwordSession(e *estate, lifetime time.Duration) credentials.Credentials {
	issued := e.startSession("", lifetime)

	return credentials.Credentials{Method: credentials.MethodPassword, AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken}
}

// oauthSession is a session kept as a browser login would have kept it,
// whose access token runs out in lifetime.
func oauthSession(e *estate, lifetime time.Duration) credentials.Credentials {
	e.knowClient("client-1", "http://127.0.0.1:1/callback")
	issued := e.startSession("client-1", lifetime)

	return credentials.Credentials{Method: credentials.MethodOAuth, AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken, ClientID: "client-1", TokenEndpoint: e.URL + "/as/token"}
}

func failing(operation string) func(string, string) error {
	return func(asked, _ string) error {
		if asked == operation {
			return errors.New(operation + " failed")
		}

		return nil
	}
}

func TestTokenSource(t *testing.T) {
	t.Parallel()

	t.Run("a token that lasts is handed over as it is kept", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Token(context.Background())

		require.NoError(t, err)
		assert.Equal(t, kept.AccessToken, got)
		assert.Zero(t, e.called("POST /api/auth/token/refresh"))
	})

	t.Run("a session is read once while its token lasts", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})
		h := newHarness(t, e, store)

		for range 3 {
			got, err := h.TokenSource("test").Token(context.Background())
			require.NoError(t, err)
			assert.Equal(t, kept.AccessToken, got)
		}

		assert.Equal(t, 1, store.Gets())
	})

	t.Run("a session another process renewed is read again before it is renewed", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})
		h := newHarness(t, e, store)

		_, err := h.TokenSource("test").Token(context.Background())
		require.NoError(t, err)

		elsewhere := passwordSession(e, time.Minute)
		require.NoError(t, store.Set("test", elsewhere))

		got, err := h.TokenSource("test").Renew(context.Background(), kept.AccessToken)

		require.NoError(t, err)
		assert.Equal(t, elsewhere.AccessToken, got)
		assert.Zero(t, e.called("POST /api/auth/token/refresh"))
	})

	t.Run("a session another process logged out of is not logged in", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		store := mock.NewStore(map[string]credentials.Credentials{"test": kept})
		h := newHarness(t, e, store)

		_, err := h.TokenSource("test").Token(context.Background())
		require.NoError(t, err)

		require.NoError(t, store.Delete("test"))

		_, err = h.TokenSource("test").Renew(context.Background(), kept.AccessToken)

		assert.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("a session logged in to, renewed or logged out of here is what is handed over", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		store := mock.NewStore(nil)
		h := newHarness(t, e, store)

		require.NoError(t, h.LoginWithPassword(context.Background(), "test", "jane", "secret"))

		got, err := h.TokenSource("test").Token(context.Background())
		require.NoError(t, err)
		assert.Zero(t, store.Gets(), "what was just kept is not read back")
		assert.Equal(t, h.stored(t).AccessToken, got)

		renewed, err := h.TokenSource("test").Renew(context.Background(), got)
		require.NoError(t, err)
		assert.NotEqual(t, got, renewed)

		again, err := h.TokenSource("test").Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, renewed, again)

		require.NoError(t, h.Logout(context.Background(), "test"))

		_, err = h.TokenSource("test").Token(context.Background())
		assert.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("a token about to run out is renewed first, at the API for a password session", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, 10*time.Second)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Token(context.Background())

		require.NoError(t, err)
		assert.NotEqual(t, kept.AccessToken, got)
		assert.Equal(t, 1, e.called("POST /api/auth/token/refresh"))

		renewed := h.stored(t)
		assert.Equal(t, got, renewed.AccessToken)
		assert.NotEqual(t, kept.RefreshToken, renewed.RefreshToken)
		assert.Equal(t, credentials.MethodPassword, renewed.Method)
	})

	t.Run("a token about to run out is renewed first, at the token endpoint by its application for an oauth session", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := oauthSession(e, 10*time.Second)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Token(context.Background())

		require.NoError(t, err)
		assert.NotEqual(t, kept.AccessToken, got)
		assert.Equal(t, 1, e.called("POST /as/token"))
		assert.Zero(t, e.called("POST /api/auth/token/refresh"))

		renewed := h.stored(t)
		assert.Equal(t, got, renewed.AccessToken)
		assert.NotEqual(t, kept.RefreshToken, renewed.RefreshToken)
		assert.Equal(t, "client-1", renewed.ClientID)
		assert.Equal(t, kept.TokenEndpoint, renewed.TokenEndpoint)
	})

	t.Run("a renewal that hands over no refresh token keeps the one there was", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t, func(e *estate) { e.noRefreshToken = true })
		kept := passwordSession(e, time.Second)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		_, err := h.TokenSource("test").Token(context.Background())

		require.NoError(t, err)
		assert.Equal(t, kept.RefreshToken, h.stored(t).RefreshToken)
	})

	t.Run("a token the API refused is renewed", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Renew(context.Background(), kept.AccessToken)

		require.NoError(t, err)
		assert.NotEqual(t, kept.AccessToken, got)
		assert.Equal(t, 1, e.called("POST /api/auth/token/refresh"))
	})

	t.Run("a token renewed meanwhile is handed over rather than renewed again", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Minute)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Renew(context.Background(), "an older token")

		require.NoError(t, err)
		assert.Equal(t, kept.AccessToken, got)
		assert.Zero(t, e.called("POST /api/auth/token/refresh"))
	})

	t.Run("a token renewed meanwhile that runs out too is renewed", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Second)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got, err := h.TokenSource("test").Renew(context.Background(), "an older token")

		require.NoError(t, err)
		assert.NotEqual(t, kept.AccessToken, got)
		assert.Equal(t, 1, e.called("POST /api/auth/token/refresh"))
	})

	t.Run("tokens asked for at once are renewed once", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := passwordSession(e, time.Second)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		got := make([]string, 20)

		var wait sync.WaitGroup
		for i := range got {
			wait.Go(func() {
				var err error
				if i%2 == 0 {
					got[i], err = h.TokenSource("test").Token(context.Background())
				} else {
					got[i], err = h.TokenSource("test").Renew(context.Background(), kept.AccessToken)
				}

				assert.NoError(t, err)
			})
		}

		wait.Wait()

		assert.Equal(t, 1, e.called("POST /api/auth/token/refresh"))
		for _, token := range got {
			assert.Equal(t, h.stored(t).AccessToken, token)
		}
	})

	t.Run("a context nobody is logged in to is not logged in when nobody is there to log in", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		_, err := h.TokenSource("test").Token(context.Background())
		assert.ErrorIs(t, err, ErrNotLoggedIn)
		assert.EqualError(t, err, "not logged in")

		_, err = h.TokenSource("test").Renew(context.Background(), "rejected")
		assert.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("a context nobody is logged in to is logged in to in the browser by itself when somebody is there", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil), WithInteractive(true))

		got, err := h.TokenSource("test").Token(context.Background())

		require.NoError(t, err)
		assert.Equal(t, h.stored(t).AccessToken, got)
		assert.Equal(t, credentials.MethodOAuth, h.stored(t).Method)
		assert.Contains(t, h.prompt.String(), `Not logged in to "test" (`+e.URL+`): logging in.`)
	})

	t.Run("a session whose account is refused is forgotten, and not logged in to again", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		store := mock.NewStore(map[string]credentials.Credentials{"test": {Method: credentials.MethodPassword, AccessToken: expiring("gone", 0), RefreshToken: blockedRefreshToken}})
		h := newHarness(t, e, store, WithInteractive(true))

		_, err := h.TokenSource("test").Token(context.Background())

		assert.ErrorIs(t, err, ErrAccountRefused)
		assert.NotErrorIs(t, err, ErrNotLoggedIn, "signing in again would be refused too")
		assert.EqualError(t, err, "your account may not sign in: your account has been blocked, you can reach us through the contact form")
		assert.NotContains(t, h.prompt.String(), "logging in")

		_, err = store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("a session the estate will not renew is forgotten, and not logged in when nobody is there to log in", func(t *testing.T) {
		t.Parallel()

		for name, sessionOf := range map[string]func(*estate) credentials.Credentials{
			"password": func(*estate) credentials.Credentials {
				return credentials.Credentials{Method: credentials.MethodPassword, AccessToken: expiring("gone", 0), RefreshToken: "unknown"}
			},
			"oauth": func(e *estate) credentials.Credentials {
				return credentials.Credentials{Method: credentials.MethodOAuth, AccessToken: expiring("gone", 0), RefreshToken: "unknown", ClientID: "client-1", TokenEndpoint: e.URL + "/as/token"}
			},
			"unknown": func(*estate) credentials.Credentials {
				return credentials.Credentials{Method: "magic", AccessToken: expiring("gone", 0)}
			},
			"oauth without its token endpoint": func(*estate) credentials.Credentials {
				return credentials.Credentials{Method: credentials.MethodOAuth, AccessToken: expiring("gone", 0), ClientID: "client-1"}
			},
		} {
			e := newEstate(t)
			store := mock.NewStore(map[string]credentials.Credentials{"test": sessionOf(e)})
			h := newHarness(t, e, store)

			_, err := h.TokenSource("test").Token(context.Background())

			assert.ErrorIs(t, err, ErrNotLoggedIn, name)
			assert.ErrorIs(t, err, ErrSessionEnded, name)
			assert.EqualError(t, err, "the session has expired or ended: not logged in", name)

			_, err = store.Get("test")
			assert.ErrorIs(t, err, credentials.ErrNotFound, name)
		}
	})

	t.Run("a session the estate will not renew is logged in to again in the browser by itself when somebody is there", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		rejected := expiring("gone", time.Minute)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": {Method: credentials.MethodPassword, AccessToken: rejected, RefreshToken: "unknown"}}), WithInteractive(true))

		got, err := h.TokenSource("test").Renew(context.Background(), rejected)

		require.NoError(t, err)
		assert.Equal(t, h.stored(t).AccessToken, got)
		assert.Equal(t, credentials.MethodOAuth, h.stored(t).Method)
		assert.Contains(t, h.prompt.String(), `The session of "test" (`+e.URL+`) has ended: logging in again.`)
	})

	t.Run("a renewal that fails otherwise is an error, and the session is kept", func(t *testing.T) {
		t.Parallel()

		for name, prepare := range map[string]func(*estate){
			"an estate failing": func(e *estate) {
				e.fail["POST /api/auth/token/refresh"] = failure{status: http.StatusInternalServerError}
			},
			"an estate answering no token": func(e *estate) { e.noAccessToken = true },
		} {
			e := newEstate(t, prepare)
			kept := passwordSession(e, time.Second)
			store := mock.NewStore(map[string]credentials.Credentials{"test": kept})
			h := newHarness(t, e, store, WithInteractive(true))

			_, err := h.TokenSource("test").Token(context.Background())

			assert.ErrorContains(t, err, `renewing the session of "test": `, name)
			assert.Equal(t, kept, h.stored(t), name)
		}
	})

	t.Run("a password session of a context whose api is no url is an error", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"broken": passwordSession(e, time.Second)}))
		h.config.Contexts["broken"] = config.Context{Name: "broken", API: "not a url"}

		_, err := h.TokenSource("broken").Token(context.Background())

		assert.ErrorContains(t, err, `renewing the session of "broken": `)
	})

	t.Run("an oauth session whose token endpoint is no url is an error", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		kept := credentials.Credentials{Method: credentials.MethodOAuth, AccessToken: expiring("s", 0), RefreshToken: "r", ClientID: "client-1", TokenEndpoint: "http://%zz"}
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"test": kept}))

		_, err := h.TokenSource("test").Token(context.Background())

		assert.ErrorContains(t, err, `renewing the session of "test": `)
	})

	t.Run("a session of a context that is no more is an error", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(map[string]credentials.Credentials{"ghost": passwordSession(e, time.Second)}))

		_, err := h.TokenSource("ghost").Token(context.Background())
		assert.ErrorIs(t, err, config.ErrNoSuchContext)

		_, err = newHarness(t, e, mock.NewStore(nil), WithInteractive(true)).TokenSource("ghost").Token(context.Background())
		assert.ErrorIs(t, err, config.ErrNoSuchContext)
	})

	t.Run("a login in the browser that fails is the error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t, func(e *estate) { e.deny = true }), mock.NewStore(nil), WithInteractive(true))

		_, err := h.TokenSource("test").Token(context.Background())

		assert.ErrorContains(t, err, "the login was refused")
	})

	t.Run("a store that fails is an error", func(t *testing.T) {
		t.Parallel()

		for _, operation := range []string{"get", "set", "delete"} {
			e := newEstate(t)

			kept := passwordSession(e, time.Second)
			if operation == "delete" {
				kept.RefreshToken = "unknown"
			}

			store := mock.NewStore(map[string]credentials.Credentials{"test": kept})
			store.Fail = failing(operation)

			_, err := newHarness(t, e, store).TokenSource("test").Token(context.Background())

			assert.EqualError(t, err, operation+" failed")
		}
	})

	t.Run("a token given up on while another is renewed stops", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		release, err := h.hold(context.Background(), "test")
		require.NoError(t, err)
		t.Cleanup(release)

		_, err = h.TokenSource("test").Token(canceled())

		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestRefused(t *testing.T) {
	t.Parallel()

	assert.True(t, refused(errUnrenewable))
	assert.True(t, refused(&oauthError{StatusCode: http.StatusBadRequest}))
	assert.True(t, refused(&oauthError{StatusCode: http.StatusUnauthorized}))
	assert.False(t, refused(&oauthError{StatusCode: http.StatusInternalServerError}))
	assert.False(t, refused(errors.New("connection refused")))
	assert.False(t, refused(nil))
}
