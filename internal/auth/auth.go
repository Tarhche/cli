// Package auth signs the CLI in to a context and keeps its session fresh: in
// a browser, through the estate's OAuth 2.1 authorization server (an
// authorization code with PKCE, answered on the loopback interface), or with
// a password; renewing a token before it runs out and once the API refuses
// it, and signing in again by itself when a session is gone and it may.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/browser"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/version"
)

// ErrNotLoggedIn is a context nobody is signed in to, or whose session is
// gone, when signing in again is not something to do by itself.
var ErrNotLoggedIn = errors.New("not logged in")

// ErrSessionEnded is a session its estate would not renew: it ran out, or was
// signed out of. It is ErrNotLoggedIn as well.
var ErrSessionEnded = fmt.Errorf("the session has expired or ended: %w", ErrNotLoggedIn)

// ErrAccountRefused is an account its estate refuses a session to, blocked
// or gone: signing in again would be refused too.
var ErrAccountRefused = errors.New("the account is refused")

// AccountRefusedError is a session's renewal refused for its account, with
// why, as the estate says it.
type AccountRefusedError struct {
	Reason string
}

func (e *AccountRefusedError) Error() string {
	return "your account may not sign in: " + e.Reason
}

// Is reports whether target is ErrAccountRefused, which every refusal of an
// account is.
func (e *AccountRefusedError) Is(target error) bool {
	return target == ErrAccountRefused
}

// errNotRenewed is a token refused where it is not renewed.
var errNotRenewed = errors.New("the token was refused")

const (
	// renewBefore is how long before it runs out an access token is
	// renewed rather than used.
	renewBefore = 30 * time.Second

	// endSessionTimeout bounds how long a logout tries to end the session on
	// its estate.
	endSessionTimeout = 10 * time.Second
)

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

	// Permissions are every action the session's roles allow somewhere, as
	// its access token says: a hint of what to offer.
	Permissions []string `json:"permissions,omitempty"`
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
	language    string

	// loginTimeout bounds how long a browser login waits for the user.
	loginTimeout time.Duration

	// env reads the environment, which the language a device says it
	// speaks is read off.
	env func(string) string

	lock     sync.Mutex
	held     map[string]chan struct{}
	sessions map[string]credentials.Credentials
	configs  sync.Mutex
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

// WithLanguage asks the API to answer in language, which is what the reasons
// it refuses a login are worded in.
func WithLanguage(language string) Option {
	return func(m *Manager) { m.language = language }
}

// New is a manager of the contexts of cfg, keeping their sessions in store.
// It opens pages in the user's default browser unless told otherwise.
func New(cfg *config.File, store credentials.Store, options ...Option) *Manager {
	m := &Manager{
		config:       cfg,
		store:        store,
		http:         &http.Client{Timeout: api.DefaultTimeout},
		browser:      browser.Open,
		loginTimeout: loginTimeout,
		env:          os.Getenv,
	}

	for _, option := range options {
		option(m)
	}

	return m
}

// Logout ends the session of a context, where it is kept and on the estate
// where it can: forgetting it is what a logout is, so nothing the estate
// answers fails it.
func (m *Manager) Logout(ctx context.Context, contextName string) error {
	release, err := m.hold(ctx, contextName)
	if err != nil {
		return err
	}

	defer release()

	kept, readErr := m.store.Get(contextName)

	if err := m.store.Delete(contextName); err != nil {
		return err
	}

	m.forget(contextName)

	if readErr == nil {
		m.endSession(ctx, contextName, kept)
	}

	return nil
}

// Status is who a context is signed in as, as its session is kept: the
// estate is not asked.
func (m *Manager) Status(ctx context.Context, contextName string) (Status, error) {
	kept, err := m.store.Get(contextName)
	if errors.Is(err, credentials.ErrNotFound) {
		return Status{}, nil
	} else if err != nil {
		return Status{}, err
	}

	claims := claimsOf(kept.AccessToken)

	return Status{LoggedIn: true, Method: kept.Method, Subject: claims.Subject, ExpiresAt: claims.expiry(), Permissions: claims.Permissions}, nil
}

// hold waits until nothing else signs in to, out of or renews the session of
// a context, and is what lets it go.
func (m *Manager) hold(ctx context.Context, contextName string) (func(), error) {
	m.lock.Lock()
	if m.held == nil {
		m.held = make(map[string]chan struct{})
	}

	held, found := m.held[contextName]
	if !found {
		held = make(chan struct{}, 1)
		m.held[contextName] = held
	}
	m.lock.Unlock()

	select {
	case held <- struct{}{}:
		return func() { <-held }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// remembered is a context's session as it was last read or kept.
func (m *Manager) remembered(contextName string) (credentials.Credentials, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	kept, found := m.sessions[contextName]

	return kept, found
}

// remember notes what a context's session was read or kept as, so that it
// is not read again while its token lasts.
func (m *Manager) remember(contextName string, kept credentials.Credentials) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.sessions == nil {
		m.sessions = make(map[string]credentials.Credentials)
	}

	m.sessions[contextName] = kept
}

// forget forgets what a context's session was last read or kept as.
func (m *Manager) forget(contextName string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.sessions, contextName)
}

// endSession ends a session on its estate, with its own token: a session
// may always end itself. One about to run out is renewed first.
func (m *Manager) endSession(ctx context.Context, contextName string, kept credentials.Credentials) {
	current, err := m.config.Context(contextName)
	if err != nil {
		return
	}

	session := claimsOf(kept.AccessToken).Session
	if len(session) == 0 {
		session = claimsOf(kept.RefreshToken).Session
	}

	if len(session) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, endSessionTimeout)
	defer cancel()

	token := kept.AccessToken
	if expiresWithin(token, renewBefore) {
		if renewed, err := m.renew(ctx, current, kept); err == nil {
			token = renewed.AccessToken
		}
	}

	client, err := m.client(current.API, token)
	if err != nil {
		return
	}

	_ = client.Delete(ctx, "/api/dashboard/profile/sessions/"+url.PathEscape(session), nil, nil)
}

// client is a client of an estate's API, signing its requests with token
// when there is one.
func (m *Manager) client(base, token string) (*api.Client, error) {
	options := []api.Option{api.WithHTTPClient(m.http), api.WithUserAgent(version.UserAgent())}
	if len(token) > 0 {
		options = append(options, api.WithTokens(fixedToken(token)))
	}

	if len(m.language) > 0 {
		options = append(options, api.WithLanguage(m.language))
	}

	return api.New(base, options...)
}

// say tells the user what is going on, on the prompt.
func (m *Manager) say(format string, args ...any) {
	if m.prompt == nil {
		return
	}

	_, _ = fmt.Fprintf(m.prompt, format, args...)
}

// fixedToken signs requests with one token, which is not renewed when it is
// refused.
type fixedToken string

func (t fixedToken) Token(context.Context) (string, error) {
	return string(t), nil
}

func (t fixedToken) Renew(context.Context, string) (string, error) {
	return "", errNotRenewed
}
