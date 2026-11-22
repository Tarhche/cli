// Package commandtest runs commands in tests: against an API a test serves,
// signed in with a token that is always good, writing to buffers.
package commandtest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
)

// Token is the access token a harness signs its requests with.
const Token = "test-token"

// Harness is a Cli whose API is a test's.
type Harness struct {
	Cli    *command.Cli
	In     *strings.Reader
	Out    *bytes.Buffer
	Err    *bytes.Buffer
	Server *httptest.Server
	Config *config.File
}

// New is a harness whose API is handler, whose configuration is in a
// directory of the test's own, and whose current context points at the API.
// A nil handler answers every request 404.
func New(t testing.TB, handler http.Handler, options ...command.Option) *Harness {
	t.Helper()

	if handler == nil {
		handler = http.NotFoundHandler()
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := cfg.SetContext(config.Context{Name: "test", API: server.URL, Ingress: server.URL}); err != nil {
		t.Fatal(err)
	}

	if err := cfg.Use("test"); err != nil {
		t.Fatal(err)
	}

	client, err := api.New(server.URL, api.WithTokens(Tokens(Token)))
	if err != nil {
		t.Fatal(err)
	}

	h := &Harness{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Server: server, Config: cfg}

	defaults := []command.Option{command.WithConfig(cfg), command.WithTokens(Tokens(Token)), command.WithClient(client)}
	h.Cli = command.NewCli(h.In, h.Out, h.Err, append(defaults, options...)...)

	return h
}

// Run runs cmd with args, through a console as the CLI would, and is the
// status it exits with.
func (h *Harness) Run(cmd console.Command, args ...string) int {
	return h.RunContext(context.Background(), cmd, args...)
}

// RunContext runs cmd with args under ctx.
func (h *Harness) RunContext(ctx context.Context, cmd console.Command, args ...string) int {
	c := console.New(command.Name, "", h.Out, h.Err).Register(cmd)

	return c.Run(ctx, append([]string{command.Name, cmd.Name()}, args...))
}

// Tokens is a token source that always hands over token.
type Tokens string

var _ api.TokenSource = Tokens("")

// Token is the token.
func (t Tokens) Token(context.Context) (string, error) {
	return string(t), nil
}

// Renew is the token again, which is never refused.
func (t Tokens) Renew(context.Context, string) (string, error) {
	return string(t), nil
}
