package desktop_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/desktop"
)

// testContext is the context a harness talks to.
const testContext = "test"

// opener opens the Cli of a context as the harness's own is: on its
// configuration, signed in with its token.
func opener(h *commandtest.Harness) desktop.Opener {
	return openerWith(h, commandtest.Token)
}

// openerWith opens the Cli of a context on the harness's configuration,
// signed in with token.
func openerWith(h *commandtest.Harness, token string) desktop.Opener {
	return func(contextName string) *command.Cli {
		cli := command.NewCli(h.In, h.Out, h.Err, command.WithConfig(h.Config), command.WithTokens(commandtest.Tokens(token)))
		cli.Globals.Context = contextName

		return cli
	}
}

// newWorkspace is a workspace talking to the test context, whose API is
// handler.
func newWorkspace(t *testing.T, handler http.Handler) (*desktop.Workspace, *commandtest.Harness) {
	t.Helper()

	h := commandtest.New(t, handler)

	return desktop.NewWorkspace(testContext, opener(h)), h
}

// withWebsite is w, its context naming web as its website.
func withWebsite(t *testing.T, w *desktop.Workspace, web string) *desktop.Workspace {
	t.Helper()

	require.NoError(t, w.Change(func(cfg *config.File, current string) (string, error) {
		c, err := cfg.Context(current)
		if err != nil {
			return "", err
		}

		c.Web = web

		return "", cfg.SetContext(c)
	}))

	return w
}

// newWorkspaceGranting is a workspace talking to the test context, whose API
// is handler, signed in with a token whose roles allow permissions.
func newWorkspaceGranting(t *testing.T, handler http.Handler, permissions ...string) *desktop.Workspace {
	t.Helper()

	return desktop.NewWorkspace(testContext, openerWith(commandtest.New(t, handler), tokenGranting(t, permissions...)))
}

// tokenGranting is an access token, signed by nobody, whose roles allow
// permissions: the desktop reads its claims without verifying it, as the
// estate verifies every token it is handed.
func tokenGranting(t *testing.T, permissions ...string) string {
	t.Helper()

	claims, err := json.Marshal(map[string]any{"sub": "u-1", "permissions": permissions})
	require.NoError(t, err)

	return "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".unsigned"
}

// failingTokens is a token source that cannot hand over a token.
type failingTokens struct {
	err error
}

func (f failingTokens) Token(context.Context) (string, error) {
	return "", f.err
}

func (f failingTokens) Renew(context.Context, string) (string, error) {
	return "", f.err
}

// workspaceFailing is a workspace talking to the test context, whose API is
// handler, whose session cannot be had: err is why.
func workspaceFailing(t *testing.T, handler http.Handler, err error) *desktop.Workspace {
	t.Helper()

	h := commandtest.New(t, handler)

	return desktop.NewWorkspace(testContext, func(contextName string) *command.Cli {
		cli := command.NewCli(h.In, h.Out, h.Err, command.WithConfig(h.Config), command.WithTokens(failingTokens{err: err}))
		cli.Globals.Context = contextName

		return cli
	})
}

// brokenConfig is a directory whose configuration cannot be read.
func brokenConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileName), []byte(`{`), 0o600))

	return dir
}

// brokenWorkspace is a workspace whose configuration cannot be read, nor
// therefore which context it talks to.
func brokenWorkspace(t *testing.T) *desktop.Workspace {
	t.Helper()

	dir := brokenConfig(t)

	return desktop.NewWorkspace(testContext, func(string) *command.Cli {
		cli := command.NewCli(nil, io.Discard, io.Discard)
		cli.Globals.ConfigDir = dir

		return cli
	})
}

// request is what an API was asked.
type request struct {
	Method string
	Path   string
	Query  string
	Body   string
	Token  string
}

// recorder is an API that answers every request alike and remembers what it
// was asked.
type recorder struct {
	status int
	answer string

	lock  sync.Mutex
	asked []request
}

func newRecorder(status int, answer string) *recorder {
	return &recorder{status: status, answer: answer}
}

func (r *recorder) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)

	r.lock.Lock()
	r.asked = append(r.asked, request{
		Method: req.Method,
		Path:   req.URL.EscapedPath(),
		Query:  req.URL.RawQuery,
		Body:   string(body),
		Token:  req.Header.Get("Authorization"),
	})
	r.lock.Unlock()

	if len(r.answer) > 0 {
		rw.Header().Set("Content-Type", "application/json")
	}

	rw.WriteHeader(r.status)
	_, _ = io.WriteString(rw, r.answer)
}

// last is the last request the API was asked.
func (r *recorder) last(t *testing.T) request {
	t.Helper()

	r.lock.Lock()
	defer r.lock.Unlock()

	require.NotEmpty(t, r.asked, "the API was asked nothing")

	return r.asked[len(r.asked)-1]
}

// count is how many requests the API was asked.
func (r *recorder) count() int {
	r.lock.Lock()
	defer r.lock.Unlock()

	return len(r.asked)
}

// jsonOf is value as JSON.
func jsonOf(t *testing.T, value any) string {
	t.Helper()

	content, err := json.Marshal(value)
	require.NoError(t, err)

	return string(content)
}
