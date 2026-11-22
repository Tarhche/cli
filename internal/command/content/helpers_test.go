package content

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/command/commandtest"
	"github.com/Tarhche/cli/internal/config"
)

// reply is what the fake API answers a request with.
type reply struct {
	status int
	body   string
}

// sent is a request the fake API was sent.
type sent struct {
	Method string
	Path   string
	Query  string
	Body   string
	Header http.Header
}

// fakeAPI answers each request by "METHOD /path?query", else by "METHOD
// /path", and keeps what it was sent. A route with more than one reply
// answers them in turn, its last again and again; any other route is
// answered 404.
type fakeAPI struct {
	lock    sync.Mutex
	replies map[string][]reply
	sent    []sent
}

func newFakeAPI(replies map[string][]reply) *fakeAPI {
	return &fakeAPI{replies: replies}
}

func (f *fakeAPI) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.lock.Lock()

	f.sent = append(f.sent, sent{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body), Header: r.Header.Clone()})

	route := r.Method + " " + r.URL.Path
	if _, found := f.replies[route+"?"+r.URL.RawQuery]; found && len(r.URL.RawQuery) > 0 {
		route += "?" + r.URL.RawQuery
	}

	answer := reply{status: http.StatusNotFound}
	if replies := f.replies[route]; len(replies) > 0 {
		answer = replies[0]
		if len(replies) > 1 {
			f.replies[route] = replies[1:]
		}
	}

	f.lock.Unlock()

	if answer.status == 0 {
		answer.status = http.StatusOK
	}

	if len(answer.body) > 0 {
		rw.Header().Set("Content-Type", "application/json")
	}

	rw.WriteHeader(answer.status)
	_, _ = io.WriteString(rw, answer.body)
}

// requests are what the fake API was sent, in order.
func (f *fakeAPI) requests() []sent {
	f.lock.Lock()
	defer f.lock.Unlock()

	return append([]sent(nil), f.sent...)
}

// routes are the requests the fake API was sent, each as "METHOD
// /path?query".
func (f *fakeAPI) routes() []string {
	var routes []string

	for _, request := range f.requests() {
		route := request.Method + " " + request.Path
		if len(request.Query) > 0 {
			route += "?" + request.Query
		}

		routes = append(routes, route)
	}

	return routes
}

// answer is a route answered once, 200 with body.
func answer(body string) []reply {
	return []reply{{status: http.StatusOK, body: body}}
}

// answered is a route answered once with status and body.
func answered(status int, body string) []reply {
	return []reply{{status: status, body: body}}
}

// harness is a harness whose API is api.
func harness(t *testing.T, api http.Handler) *commandtest.Harness {
	t.Helper()

	return commandtest.New(t, api)
}

// clientless is a Cli that cannot build a client, as its context does not
// exist, and the streams it writes to.
func clientless(t *testing.T) (*command.Cli, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}

	cli := command.NewCli(strings.NewReader(""), out, errOut, command.WithConfig(cfg))
	cli.Globals.Context = "missing"

	return cli, out, errOut
}

// runOn runs cmd with args through a console writing to out and errOut, and
// is the status it exits with.
func runOn(cmd console.Command, out, errOut io.Writer, args ...string) int {
	return console.New(command.Name, "", out, errOut).Register(cmd).Run(context.Background(), append([]string{command.Name, cmd.Name()}, args...))
}

// failingWriter is a writer that cannot be written to.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("closed")
}

// unwritable checks that a command built on a Cli whose API is api and
// whose output cannot be written says so, and fails.
func unwritable(t *testing.T, api http.Handler, build func(cli *command.Cli) console.Command, args ...string) {
	t.Helper()

	h := harness(t, api)

	client, err := h.Cli.Client(context.Background())
	require.NoError(t, err)

	cli := command.NewCli(h.In, failingWriter{}, h.Err, command.WithConfig(h.Config), command.WithClient(client))

	status := runOn(build(cli), failingWriter{}, h.Err, args...)

	require.Equal(t, console.ExitFailure, status, h.Err.String())
	require.Contains(t, h.Err.String(), "closed")
}

// noClient checks that a command that cannot build a client says why and
// fails.
func noClient(t *testing.T, build func(cli *command.Cli) console.Command, args ...string) {
	t.Helper()

	cli, out, errOut := clientless(t)

	status := runOn(build(cli), out, errOut, args...)

	require.Equal(t, console.ExitFailure, status, errOut.String())
	require.Contains(t, errOut.String(), "no such context")
}
