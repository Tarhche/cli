package system

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/credentials/mock"
)

const (
	profileAnswer      = `{"uuid":"user-1","name":"Jane Doe","email":"jane@example.com","username":"jane","language_code":"en"}`
	rolesAnswer        = `{"items":[{"uuid":"role-1","translations":{"en":{"title":"Administrator"},"fa":{"title":"مدیر"}},"statements":[]},{"uuid":"role-2","translations":{"en":{"title":"Writer"}},"statements":[]}]}`
	subscriptionAnswer = `{"subscription":{"uuid":"plan-1","names":{"en":"Pro","fa":"ویژه"},"quotas":{"vms":{"max":5,"cpus":2,"memory":1073741824,"disk":10737418240,"enforcement":{"mode":"soft"}},"snapshots":{"max":10,"enforcement":{"mode":"soft"}},"uploads":{"max_size":1048576,"enforcement":{"mode":"soft"}}}},"usage":{"vms":2,"snapshots":1,"files":3},"over":[]}`
)

// visits records the routes an estate was asked, in order.
type visits struct {
	lock   sync.Mutex
	routes []string
}

func (v *visits) add(route string) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.routes = append(v.routes, route)
}

// asked are the routes asked, in order.
func (v *visits) asked() []string {
	v.lock.Lock()
	defer v.lock.Unlock()

	return append([]string(nil), v.routes...)
}

// estate answers the routes of an estate the system commands ask, as one
// whose user approves the CLI and signs in as jane with secret would, and
// records what it was asked in seen. What routes holds takes the place of
// any of them.
func estate(seen *visits, routes map[string]http.HandlerFunc) http.Handler {
	var lock sync.Mutex

	clients := make(map[string]string)

	answers := map[string]http.HandlerFunc{
		"POST /oauth/register": func(rw http.ResponseWriter, r *http.Request) {
			var asked struct {
				RedirectURIs []string `json:"redirect_uris"`
			}

			if err := json.NewDecoder(r.Body).Decode(&asked); err != nil || len(asked.RedirectURIs) != 1 {
				rw.WriteHeader(http.StatusBadRequest)

				return
			}

			lock.Lock()
			id := fmt.Sprintf("client-%d", len(clients)+1)
			clients[id] = asked.RedirectURIs[0]
			lock.Unlock()

			answer(rw, http.StatusCreated, `{"client_id":"`+id+`"}`)
		},
		"GET /oauth/authorize": func(rw http.ResponseWriter, r *http.Request) {
			lock.Lock()
			redirectURI, known := clients[r.URL.Query().Get("client_id")]
			lock.Unlock()

			if !known {
				answer(rw, http.StatusUnauthorized, `{"error":"invalid_client"}`)

				return
			}

			http.Redirect(rw, r, redirectURI+"?"+url.Values{"code": {"code-1"}, "state": {r.URL.Query().Get("state")}}.Encode(), http.StatusFound)
		},
		"POST /oauth/token": func(rw http.ResponseWriter, _ *http.Request) {
			answer(rw, http.StatusOK, `{"access_token":"`+sessionToken("session-1")+`","refresh_token":"refresh","token_type":"Bearer","expires_in":180}`)
		},
		"POST /api/auth/login": func(rw http.ResponseWriter, r *http.Request) {
			var asked struct {
				Identity string `json:"identity"`
				Password string `json:"password"`
			}

			if err := json.NewDecoder(r.Body).Decode(&asked); err != nil || asked.Identity != "jane" || asked.Password != "secret" {
				answer(rw, http.StatusBadRequest, `{"errors":{"identity":"invalid identity or password"}}`)

				return
			}

			answer(rw, http.StatusOK, `{"access_token":"`+sessionToken("session-2")+`","refresh_token":"refresh"}`)
		},
		"GET /api/dashboard/profile": func(rw http.ResponseWriter, _ *http.Request) {
			answer(rw, http.StatusOK, profileAnswer)
		},
		"GET /api/dashboard/profile/roles": func(rw http.ResponseWriter, _ *http.Request) {
			answer(rw, http.StatusOK, rolesAnswer)
		},
		"GET /api/dashboard/profile/subscription": func(rw http.ResponseWriter, _ *http.Request) {
			answer(rw, http.StatusOK, subscriptionAnswer)
		},
		"DELETE /api/dashboard/profile/sessions/{uuid}": func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(http.StatusNoContent)
		},
	}

	maps.Copy(answers, routes)

	mux := http.NewServeMux()
	for pattern, handler := range answers {
		mux.HandleFunc(pattern, handler)
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if seen != nil {
			seen.add(r.Method + " " + r.URL.Path)
		}

		mux.ServeHTTP(rw, r)
	})
}

func answer(rw http.ResponseWriter, status int, body string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_, _ = io.WriteString(rw, body)
}

// sessionToken is an access token of session, which lasts.
func sessionToken(session string) string {
	payload, _ := json.Marshal(map[string]any{"sub": "user-1", "sid": session, "exp": time.Now().Add(time.Hour).Unix()})

	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

// signedIn is a session kept for a context, as a login of method would
// have kept it.
func signedIn(method string) credentials.Credentials {
	return credentials.Credentials{Method: method, AccessToken: sessionToken("session-9"), RefreshToken: "refresh"}
}

// follow is a browser that goes where it is sent, and wherever it is sent
// on.
func follow(page string) error {
	response, err := http.Get(page)
	if err != nil {
		return err
	}

	defer func() { _ = response.Body.Close() }()

	_, err = io.Copy(io.Discard, response.Body)

	return err
}

// newHarness is a harness whose API is handler, signing in and out through
// a manager keeping its sessions in store, whose browser goes wherever it is
// sent and says what it does on the harness's errors.
func newHarness(t *testing.T, handler http.Handler, store credentials.Store, options ...auth.Option) *commandtest.Harness {
	t.Helper()

	h := commandtest.New(t, handler)

	client, err := h.Cli.Client(context.Background())
	require.NoError(t, err)

	defaults := []auth.Option{auth.WithHTTPClient(h.Server.Client()), auth.WithBrowser(follow), auth.WithPrompt(h.Err)}
	manager := auth.New(h.Config, store, append(defaults, options...)...)

	h.Cli = command.NewCli(h.In, h.Out, h.Err, command.WithConfig(h.Config), command.WithAuth(manager), command.WithClient(client), command.WithTokens(commandtest.Tokens(commandtest.Token)))

	return h
}

// withWebsite names web as the website of the harness's context, "test".
func withWebsite(t *testing.T, h *commandtest.Harness, web string) {
	t.Helper()

	current, err := h.Config.Context("test")
	require.NoError(t, err)

	current.Web = web
	require.NoError(t, h.Config.SetContext(current))
}

// unsavable is a configuration that cannot be saved, since its directory
// cannot be made, with a context "test" at api.
func unsavable(t *testing.T, api string) *config.File {
	t.Helper()

	dangling := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "nowhere", "config"), dangling))

	cfg, err := config.Load(dangling)
	require.NoError(t, err)
	require.NoError(t, cfg.SetContext(config.Context{Name: "test", API: api}))
	require.NoError(t, cfg.Use("test"))

	return cfg
}

// unreadable is a CLI whose configuration cannot be read.
func unreadable(t *testing.T, h *commandtest.Harness) *command.Cli {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte("{"), 0o600))

	cli := command.NewCli(h.In, h.Out, h.Err)
	cli.Globals.ConfigDir = dir

	return cli
}

// unwritable is the CLI of h, whose output cannot be written to.
func unwritable(h *commandtest.Harness) *command.Cli {
	return command.NewCli(h.In, failingWriter{}, h.Err, command.WithConfig(h.Config), command.WithAuth(auth.New(h.Config, mock.NewStore(nil))))
}

// emptyStore is a store keeping nothing yet.
func emptyStore() *mock.Store {
	return mock.NewStore(nil)
}
