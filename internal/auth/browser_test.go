package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

// freePort is a port on the loopback interface nothing listens on.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	return port
}

// callbackOf is where page sends the browser back to, with query.
func callbackOf(t *testing.T, page string, query url.Values) string {
	t.Helper()

	asked, err := url.Parse(page)
	require.NoError(t, err)

	back, err := url.Parse(asked.Query().Get("redirect_uri"))
	require.NoError(t, err)

	back.RawQuery = query.Encode()

	return back.String()
}

// visit asks for page as a browser would, and is the status it is
// answered with.
func visit(t *testing.T, page string) int {
	t.Helper()

	response, err := http.Get(page)
	require.NoError(t, err)

	defer func() { _ = response.Body.Close() }()

	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)

	return response.StatusCode
}

func TestManagerLoginWithBrowser(t *testing.T) {
	t.Parallel()

	t.Run("a first login registers the CLI, keeps the application and the session", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		registrations := e.registered()
		require.Len(t, registrations, 1)
		assert.Equal(t, ClientName, registrations[0].ClientName)
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, registrations[0].GrantTypes)
		assert.Equal(t, []string{"code"}, registrations[0].ResponseTypes)
		assert.Equal(t, "none", registrations[0].TokenEndpointAuthMethod)
		require.Len(t, registrations[0].RedirectURIs, 1)
		assert.Regexp(t, `^http://127\.0\.0\.1:\d+/callback$`, registrations[0].RedirectURIs[0])

		saved, err := config.Load(h.config.Dir())
		require.NoError(t, err)

		current, err := saved.Context("test")
		require.NoError(t, err)
		assert.Equal(t, &config.OAuthClient{ID: "client-1", RedirectURI: registrations[0].RedirectURIs[0]}, current.OAuthClient)

		kept := h.stored(t)
		assert.Equal(t, credentials.MethodOAuth, kept.Method)
		assert.Equal(t, "client-1", kept.ClientID)
		assert.Equal(t, e.URL+"/as/token", kept.TokenEndpoint)
		assert.Equal(t, "user-1", claimsOf(kept.AccessToken).Subject)
		assert.NotEmpty(t, kept.RefreshToken)

		assert.Contains(t, h.prompt.String(), `Opening a browser to log in to "test" (`+e.URL+`)`)
		assert.Contains(t, h.prompt.String(), e.URL+"/as/authorize?")
		assert.Contains(t, h.prompt.String(), "Waiting for the login in the browser...")
	})

	t.Run("a later login is the application kept, at its address when it is free", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))

		redirectURI := "http://127.0.0.1:" + strconv.Itoa(freePort(t)) + "/callback"
		e.knowClient("kept", redirectURI)
		require.NoError(t, h.config.SetContext(config.Context{Name: "test", API: e.URL, OAuthClient: &config.OAuthClient{ID: "kept", RedirectURI: redirectURI}}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		assert.Empty(t, e.registered())
		assert.Equal(t, "kept", h.stored(t).ClientID)
	})

	t.Run("an application whose address is taken is registered again at another", func(t *testing.T) {
		t.Parallel()

		taken, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = taken.Close() })

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))

		redirectURI := "http://" + taken.Addr().String() + "/callback"
		e.knowClient("kept", redirectURI)
		require.NoError(t, h.config.SetContext(config.Context{Name: "test", API: e.URL, OAuthClient: &config.OAuthClient{ID: "kept", RedirectURI: redirectURI}}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		registrations := e.registered()
		require.Len(t, registrations, 1)
		assert.NotEqual(t, redirectURI, registrations[0].RedirectURIs[0])

		current, err := h.config.Context("test")
		require.NoError(t, err)
		assert.Equal(t, "client-1", current.OAuthClient.ID)
		assert.Equal(t, "client-1", h.stored(t).ClientID)
	})

	t.Run("an application the estate forgot is registered again, at the same address", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))

		redirectURI := "http://127.0.0.1:" + strconv.Itoa(freePort(t)) + "/callback"
		require.NoError(t, h.config.SetContext(config.Context{Name: "test", API: e.URL, OAuthClient: &config.OAuthClient{ID: "forgotten", RedirectURI: redirectURI}}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		registrations := e.registered()
		require.Len(t, registrations, 1)
		assert.Equal(t, []string{redirectURI}, registrations[0].RedirectURIs)
		assert.Equal(t, "client-1", h.stored(t).ClientID)
	})

	t.Run("an application kept with an address that is not on the loopback interface is registered again", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))

		for _, redirectURI := range []string{"https://app.example/callback", "http://localhost:8080/callback", "http://127.0.0.1/callback", "http://%zz"} {
			require.NoError(t, h.config.SetContext(config.Context{Name: "test", API: e.URL, OAuthClient: &config.OAuthClient{ID: "elsewhere", RedirectURI: redirectURI}}))

			require.NoError(t, h.LoginWithBrowser(context.Background(), "test"), redirectURI)
		}

		assert.Len(t, e.registered(), 4)
	})

	t.Run("the default context keeps its application too", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))
		require.NoError(t, h.config.SetContext(config.Context{Name: config.DefaultContextName, API: e.URL}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), config.DefaultContextName))

		saved, err := config.Load(h.config.Dir())
		require.NoError(t, err)

		current, err := saved.Context(config.DefaultContextName)
		require.NoError(t, err)
		assert.Equal(t, "client-1", current.OAuthClient.ID)
	})

	t.Run("an estate that does not describe its authorization server is asked where the API serves it", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t, func(e *estate) { e.noMetadata = true })
		h := newHarness(t, e, mock.NewStore(nil))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		assert.Equal(t, e.URL+"/oauth/token", h.stored(t).TokenEndpoint)
		assert.Equal(t, 1, e.called("POST /oauth/register"))
		assert.Equal(t, 1, e.called("GET /oauth/authorize"))
	})

	t.Run("endpoints described as what is no url are asked where the API serves them", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t, func(e *estate) {
			e.metadata["authorization_endpoint"] = "/as/authorize"
			e.metadata["token_endpoint"] = "ftp://elsewhere/token"
			e.metadata["registration_endpoint"] = ""
		})
		h := newHarness(t, e, mock.NewStore(nil))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))

		assert.Equal(t, e.URL+"/oauth/token", h.stored(t).TokenEndpoint)
	})

	t.Run("a login the user refuses fails, and nothing is kept", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t, func(e *estate) { e.deny = true })
		store := mock.NewStore(nil)
		h := newHarness(t, e, store)

		err := h.LoginWithBrowser(context.Background(), "test")

		assert.EqualError(t, err, "the login was refused: access_denied: the request was refused")

		_, err = store.Get("test")
		assert.ErrorIs(t, err, credentials.ErrNotFound)
	})

	t.Run("a browser that cannot be opened is said, and its page still logs in", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)

		visited := make(chan error, 1)
		h := newHarness(t, e, mock.NewStore(nil), WithBrowser(func(page string) error {
			go func() { visited <- follow(page) }()

			return errors.New("no browser here")
		}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))
		require.NoError(t, <-visited)

		assert.Contains(t, h.prompt.String(), "The browser could not be opened: no browser here")
		assert.Equal(t, credentials.MethodOAuth, h.stored(t).Method)
	})

	t.Run("the browser sent back for another login, or without a code, is turned away, and the login goes on", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil), WithBrowser(func(page string) error {
			assert.Equal(t, http.StatusBadRequest, visit(t, callbackOf(t, page, url.Values{"state": {"another"}, "code": {"stolen"}})))
			assert.Equal(t, http.StatusBadRequest, visit(t, callbackOf(t, page, url.Values{"state": {stateOf(t, page)}})))
			assert.Equal(t, http.StatusNotFound, visit(t, callbackOf(t, page, nil)+"favicon.ico"))

			return follow(page)
		}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))
		assert.Equal(t, credentials.MethodOAuth, h.stored(t).Method)
	})

	t.Run("the browser sent back again once the login is over is told so", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil), WithBrowser(func(page string) error {
			if err := follow(page); err != nil {
				return err
			}

			assert.Equal(t, http.StatusConflict, visit(t, callbackOf(t, page, url.Values{"state": {stateOf(t, page)}, "code": {"again"}})))

			return nil
		}))

		require.NoError(t, h.LoginWithBrowser(context.Background(), "test"))
	})

	t.Run("a login nobody finishes times out", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil), WithBrowser(func(string) error { return nil }))
		h.loginTimeout = 50 * time.Millisecond

		err := h.LoginWithBrowser(context.Background(), "test")

		assert.EqualError(t, err, "timed out after 50ms waiting for the login in the browser")
	})

	t.Run("a login given up on stops", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil), WithBrowser(func(string) error {
			cancel()

			return nil
		}))

		assert.ErrorIs(t, h.LoginWithBrowser(ctx, "test"), context.Canceled)
	})

	t.Run("a context that does not exist is an error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		assert.ErrorIs(t, h.LoginWithBrowser(context.Background(), "missing"), config.ErrNoSuchContext)
	})

	t.Run("an api that is no url is an error", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))
		h.config.Contexts["broken"] = config.Context{Name: "broken", API: "not a url"}

		assert.EqualError(t, h.LoginWithBrowser(context.Background(), "broken"), `"not a url" is not an absolute http or https url`)
	})

	t.Run("a login given up on while another is under way stops", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		release, err := h.hold(context.Background(), "test")
		require.NoError(t, err)
		t.Cleanup(release)

		assert.ErrorIs(t, h.LoginWithBrowser(canceled(), "test"), context.Canceled)
	})

	t.Run("an estate given up on while it is asked where to log in stops", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, newEstate(t), mock.NewStore(nil))

		_, err := h.discover(canceled(), "https://backend.example")

		assert.ErrorIs(t, err, context.Canceled)
	})

	failures := []struct {
		name    string
		prepare func(*estate)
		saved   bool
		said    string
	}{
		{
			name: "a registration the estate refuses",
			prepare: func(e *estate) {
				e.fail["POST /as/register"] = failure{status: http.StatusBadRequest, body: `{"error":"invalid_redirect_uri","error_description":"no"}`}
			},
			said: "registering with the authorization server: invalid_redirect_uri: no",
		},
		{
			name:    "a registration answered with something that is not json",
			prepare: func(e *estate) { e.fail["POST /as/register"] = failure{status: http.StatusCreated, body: "created"} },
			said:    "registering with the authorization server: reading the answer of ",
		},
		{
			name:    "a registration answered without an application",
			prepare: func(e *estate) { e.noClientID = true },
			said:    "registering with the authorization server: it answered no client_id",
		},
		{
			name:    "an authorization server failing to say whether it knows the application",
			prepare: func(e *estate) { e.fail["GET /as/authorize"] = failure{status: http.StatusInternalServerError} },
			saved:   true,
			said:    "the authorization server refused the login: internal server error",
		},
		{
			name:    "an authorization server that cannot be reached",
			prepare: func(e *estate) { e.metadata["authorization_endpoint"] = "http://127.0.0.1:1/authorize" },
			saved:   true,
			said:    "connect",
		},
		{
			name: "a code the estate refuses to exchange",
			prepare: func(e *estate) {
				e.fail["POST /as/token"] = failure{status: http.StatusBadRequest, body: `{"error":"invalid_grant","error_description":"the code is not valid"}`}
			},
			said: "collecting the session: invalid_grant: the code is not valid",
		},
		{
			name:    "a code exchanged for no token",
			prepare: func(e *estate) { e.noAccessToken = true },
			said:    "the estate answered no access token",
		},
	}

	for _, f := range failures {
		t.Run(f.name+" fails the login", func(t *testing.T) {
			t.Parallel()

			e := newEstate(t, f.prepare)
			store := mock.NewStore(nil)
			h := newHarness(t, e, store)

			if f.saved {
				redirectURI := "http://127.0.0.1:" + strconv.Itoa(freePort(t)) + "/callback"
				e.knowClient("kept", redirectURI)
				require.NoError(t, h.config.SetContext(config.Context{Name: "test", API: e.URL, OAuthClient: &config.OAuthClient{ID: "kept", RedirectURI: redirectURI}}))
			}

			err := h.LoginWithBrowser(context.Background(), "test")

			require.Error(t, err)
			assert.Contains(t, err.Error(), f.said)

			_, err = store.Get("test")
			assert.ErrorIs(t, err, credentials.ErrNotFound)
		})
	}

	t.Run("a configuration that cannot be saved fails the login", func(t *testing.T) {
		t.Parallel()

		dangling := filepath.Join(t.TempDir(), "config")
		require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "nowhere", "config"), dangling))

		cfg, err := config.Load(dangling)
		require.NoError(t, err)

		e := newEstate(t)
		require.NoError(t, cfg.SetContext(config.Context{Name: "test", API: e.URL}))

		m := New(cfg, mock.NewStore(nil), WithHTTPClient(e.Client()), WithBrowser(follow))

		assert.ErrorContains(t, m.LoginWithBrowser(context.Background(), "test"), "creating")
	})

	t.Run("a context that cannot be kept as it is fails the login", func(t *testing.T) {
		t.Parallel()

		e := newEstate(t)
		h := newHarness(t, e, mock.NewStore(nil))
		h.config.Contexts["bad name"] = config.Context{Name: "bad name", API: e.URL}

		assert.ErrorIs(t, h.LoginWithBrowser(context.Background(), "bad name"), config.ErrInvalidContext)
	})

	t.Run("a session that cannot be kept fails the login", func(t *testing.T) {
		t.Parallel()

		store := mock.NewStore(nil)
		store.Fail = func(operation, _ string) error {
			if operation == "set" {
				return errors.New("keychain locked")
			}

			return nil
		}

		h := newHarness(t, newEstate(t), store)

		assert.EqualError(t, h.LoginWithBrowser(context.Background(), "test"), "keychain locked")
	})
}

// stateOf is the state page asks for a code with.
func stateOf(t *testing.T, page string) string {
	t.Helper()

	asked, err := url.Parse(page)
	require.NoError(t, err)

	return asked.Query().Get("state")
}

func TestPathOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/callback", pathOf("http://127.0.0.1:8080/callback"))
	assert.Equal(t, "/", pathOf("http://127.0.0.1:8080"))
	assert.Equal(t, "/", pathOf("http://%zz"))
}
