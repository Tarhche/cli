// Package auth signs the CLI in to a context and keeps its session fresh: in
// a browser, through the estate's OAuth 2.1 authorization server (an
// authorization code with PKCE, answered on the loopback interface), or with
// a password; renewing a token before it runs out and once the API refuses
// it, and signing in again by itself when a session is gone and it may.
package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

// ErrNotLoggedIn is a context nobody is signed in to, or whose session is
// gone, when signing in again is not something to do by itself.
var ErrNotLoggedIn = errors.New("not logged in")

// Status is who a context is signed in as.
type Status struct {
	LoggedIn bool   `json:"logged_in"`
	Method   string `json:"method,omitempty"`

	// Subject is the uuid of the user the session is theirs, as its access
	// token names them.
	Subject string `json:"subject,omitempty"`

	// ExpiresAt is when the access token runs out; the session is renewed
	// before it does.
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

// Manager signs the CLI in and out of contexts, and keeps their sessions
// fresh.
type Manager struct {
	config      *config.File
	store       credentials.Store
	http        *http.Client
	browser     func(url string) error
	prompt      io.Writer
	interactive bool
}

// Option configures a manager.
type Option func(*Manager)

// WithHTTPClient reaches the estates through h.
func WithHTTPClient(h *http.Client) Option {
	return func(m *Manager) { m.http = h }
}

// WithBrowser opens the pages a browser login asks the user to visit with
// open.
func WithBrowser(open func(url string) error) Option {
	return func(m *Manager) { m.browser = open }
}

// WithPrompt says what a browser login does, and the page it asks the user
// to visit when no browser can be opened, on w.
func WithPrompt(w io.Writer) Option {
	return func(m *Manager) { m.prompt = w }
}

// WithInteractive lets a token source sign in in a browser by itself when
// its context has no session.
func WithInteractive(interactive bool) Option {
	return func(m *Manager) { m.interactive = interactive }
}

// New is a manager of the contexts of cfg, keeping their sessions in store.
func New(cfg *config.File, store credentials.Store, options ...Option) *Manager {
	m := &Manager{config: cfg, store: store, http: &http.Client{Timeout: api.DefaultTimeout}}
	for _, option := range options {
		option(m)
	}

	return m
}

// LoginWithBrowser signs in to a context in a browser, through its
// authorization server, and keeps the session.
func (m *Manager) LoginWithBrowser(ctx context.Context, contextName string) error {
	return errors.New("auth: signing in with a browser is not implemented yet")
}

// LoginWithPassword signs in to a context with an identity (a username or
// an email) and a password, and keeps the session.
func (m *Manager) LoginWithPassword(ctx context.Context, contextName, identity, password string) error {
	return errors.New("auth: signing in with a password is not implemented yet")
}

// Logout ends the session of a context, where it is kept and on the estate
// where it can.
func (m *Manager) Logout(ctx context.Context, contextName string) error {
	return errors.New("auth: signing out is not implemented yet")
}

// Status is who a context is signed in as.
func (m *Manager) Status(ctx context.Context, contextName string) (Status, error) {
	return Status{}, errors.New("auth: status is not implemented yet")
}

// TokenSource is the source of the access tokens of a context's session.
func (m *Manager) TokenSource(contextName string) api.TokenSource {
	return unimplemented{}
}

type unimplemented struct{}

func (unimplemented) Token(context.Context) (string, error) {
	return "", ErrNotLoggedIn
}

func (unimplemented) Renew(context.Context, string) (string, error) {
	return "", ErrNotLoggedIn
}
