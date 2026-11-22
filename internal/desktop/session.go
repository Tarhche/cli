package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/config"
)

// profilePath is where the API says who a session is theirs.
const profilePath = "/api/dashboard/profile"

// Authenticator signs the app in and out of contexts: the desktop's own
// auth.Manager, which never asks anything in a terminal.
type Authenticator interface {
	LoginWithBrowser(ctx context.Context, contextName string) error
	LoginWithPassword(ctx context.Context, contextName, identity, password string) error
	Logout(ctx context.Context, contextName string) error
	Status(ctx context.Context, contextName string) (auth.Status, error)
}

// Session is how the frontend reaches the contexts the app can talk to, and
// the session it has in the one it talks to.
type Session struct {
	workspace *Workspace
	auth      Authenticator
}

// NewSession is the session service of workspace, signing in and out
// through authenticator.
func NewSession(workspace *Workspace, authenticator Authenticator) *Session {
	return &Session{workspace: workspace, auth: authenticator}
}

// ContextInfo is a context as the frontend shows it.
type ContextInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	API         string `json:"api"`
	Ingress     string `json:"ingress"`

	// Current says it is the one the app talks to.
	Current bool `json:"current"`
}

// NewContext is a context to create.
type NewContext struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	API         string `json:"api"`
	Ingress     string `json:"ingress"`
}

// Status is the session the app has in the context it talks to, and, when
// reading it found it over, why.
type Status struct {
	Context  ContextInfo `json:"context"`
	LoggedIn bool        `json:"logged_in"`

	// Method is how it was signed in: oauth (a browser) or password.
	Method string `json:"method,omitempty"`

	// Subject is the uuid of whoever it is theirs, as its token names them.
	Subject string `json:"subject,omitempty"`

	// ExpiresAt is when its access token runs out; it is renewed before.
	ExpiresAt time.Time `json:"expires_at"`

	// Permissions are every action its roles allow somewhere, as its access
	// token says: a hint of what to offer, which follows the token as it is
	// renewed.
	Permissions []string `json:"permissions"`

	// User is whoever it is theirs, as their profile says, when it could be
	// read.
	User *User `json:"user,omitempty"`

	// Ended says the session reading found was over: it ran out, or was
	// signed out of, and its estate would not renew it.
	Ended bool `json:"ended,omitempty"`

	// Blocked says the estate refuses the account the session was of,
	// blocked or gone, which is signed out of; Reason is why, when it said
	// why.
	Blocked bool   `json:"blocked,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// User is whoever a session is theirs.
type User struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}

// Contexts are every context the app can talk to, the default one among
// them, by name.
func (s *Session) Contexts() ([]ContextInfo, error) {
	var contexts []ContextInfo

	err := s.workspace.Read(func(cfg *config.File, current string) error {
		for _, name := range cfg.Names() {
			if c, err := cfg.Context(name); err == nil {
				contexts = append(contexts, contextInfo(c, current))
			}
		}

		return nil
	})

	return contexts, err
}

// Current is the context the app talks to.
func (s *Session) Current() (ContextInfo, error) {
	current, err := s.workspace.Context()
	if err != nil {
		return ContextInfo{}, err
	}

	return contextInfo(current, current.Name), nil
}

// Use makes the app talk to the context of that name, and the commands too,
// as "tarhche context use" does.
func (s *Session) Use(name string) error {
	return s.workspace.Change(func(cfg *config.File, _ string) (string, error) {
		if err := cfg.Use(name); err != nil {
			return "", err
		}

		return name, nil
	})
}

// Create creates a context, refusing what is wrong with it field by field.
func (s *Session) Create(c NewContext) error {
	created := config.Context{Name: c.Name, Description: c.Description, API: c.API, Ingress: c.Ingress}

	return s.workspace.Change(func(cfg *config.File, _ string) (string, error) {
		if fields := contextFields(cfg, created); len(fields) > 0 {
			return "", InvalidError{Fields: fields}
		}

		return "", cfg.SetContext(created)
	})
}

// Remove signs out of the context of that name and forgets it. The app
// talks to the context the configuration says is current once it was
// talking to that one.
func (s *Session) Remove(ctx context.Context, name string) error {
	err := s.workspace.Read(func(cfg *config.File, _ string) error {
		if name == config.DefaultContextName {
			return config.ErrDefaultContext
		}

		_, err := cfg.Context(name)

		return err
	})
	if err != nil {
		return err
	}

	if err := s.auth.Logout(ctx, name); err != nil && !errors.Is(err, auth.ErrNotLoggedIn) {
		return err
	}

	return s.workspace.Change(func(cfg *config.File, current string) (string, error) {
		if err := cfg.RemoveContext(name); err != nil {
			return "", err
		}

		if current == name {
			return cfg.Current(), nil
		}

		return "", nil
	})
}

// Status is the session the app has in the context it talks to. A session
// the API no longer takes is none: one that ran out says it ended, and one
// whose account the estate refuses says it is blocked, and is signed out
// of. Its profile is read first, which renews it when it is about to run
// out, so that what its token says is what is answered.
func (s *Session) Status(ctx context.Context) (Status, error) {
	current, err := s.workspace.Context()
	if err != nil {
		return Status{}, err
	}

	status := Status{Context: contextInfo(current, current.Name)}

	user, err := s.profile(ctx)

	switch {
	case errors.Is(err, access.ErrBlocked):
		return s.signOutBlocked(ctx, status)
	case errors.Is(err, auth.ErrAccountRefused):
		status.Blocked = true
		status.Reason = refusalReason(err)

		return status, nil
	case errors.Is(err, auth.ErrSessionEnded):
		status.Ended = true

		return status, nil
	case loggedOut(err):
		return status, nil
	}

	session, err := s.auth.Status(ctx, current.Name)

	switch {
	case errors.Is(err, auth.ErrNotLoggedIn):
		return status, nil
	case err != nil:
		return Status{}, err
	case !session.LoggedIn:
		return status, nil
	}

	status.LoggedIn = true
	status.Method = session.Method
	status.Subject = session.Subject
	status.ExpiresAt = session.ExpiresAt
	status.Permissions = session.Permissions
	status.User = user

	return status, nil
}

// Permissions are every action the session of the context the app talks to
// may take somewhere, as the access token its requests are signed with
// says: renewed first when it is about to run out, so that a role granted
// or withdrawn since shows once it is. Nothing but that renewal asks the
// estate anything, so the frontend reads it as often as it likes, where
// reading the status would read the profile, which the estate audits; a
// session gone fails it as it fails a request.
func (s *Session) Permissions(ctx context.Context) ([]string, error) {
	granted, err := s.workspace.Granted(ctx)
	if err != nil {
		return nil, err
	}

	return append([]string{}, granted...), nil
}

// signOutBlocked signs out of the context the app talks to, whose account
// the estate refuses every request of, and says it is blocked: the session
// could do nothing any more.
func (s *Session) signOutBlocked(ctx context.Context, status Status) (Status, error) {
	if err := s.Logout(ctx); err != nil {
		return Status{}, err
	}

	status.Blocked = true

	return status, nil
}

// LoginWithBrowser signs in to the context the app talks to in a browser,
// through its authorization server, and waits until the browser is done.
func (s *Session) LoginWithBrowser(ctx context.Context) error {
	return s.login(func(contextName string) error {
		return s.auth.LoginWithBrowser(ctx, contextName)
	})
}

// LoginWithPassword signs in to the context the app talks to with an
// identity (a username or an email) and a password.
func (s *Session) LoginWithPassword(ctx context.Context, identity, password string) error {
	return s.login(func(contextName string) error {
		return s.auth.LoginWithPassword(ctx, contextName, identity, password)
	})
}

// Logout ends the session of the context the app talks to.
func (s *Session) Logout(ctx context.Context) error {
	return s.login(func(contextName string) error {
		return s.auth.Logout(ctx, contextName)
	})
}

// login signs in or out of the context the app talks to with signIn, and
// talks to it with what that left from then on.
func (s *Session) login(signIn func(contextName string) error) error {
	current, err := s.workspace.ContextName()
	if err != nil {
		return err
	}

	if err := signIn(current); err != nil {
		return err
	}

	return s.workspace.Reopen()
}

func (s *Session) profile(ctx context.Context) (*User, error) {
	client, err := s.workspace.Client(ctx)
	if err != nil {
		return nil, err
	}

	var user User
	if err := client.Get(ctx, profilePath, nil, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// loggedOut reports whether err says there is no session, or none the API
// takes.
func loggedOut(err error) bool {
	return errors.Is(err, auth.ErrNotLoggedIn) || api.IsUnauthorized(err)
}

// refusalReason is why the estate refused the account a session was of, as
// it said it, when err says.
func refusalReason(err error) string {
	if refused, isRefused := errors.AsType[*auth.AccountRefusedError](err); isRefused {
		return refused.Reason
	}

	return ""
}

func contextInfo(c config.Context, current string) ContextInfo {
	return ContextInfo{Name: c.Name, Description: c.Description, API: c.API, Ingress: c.Ingress, Current: c.Name == current}
}

// contextFields says what is wrong with each field of a context to create,
// by the configuration's own rules: a name that is taken, or that it does
// not take, and URLs it does not take.
func contextFields(cfg *config.File, c config.Context) map[string]string {
	fields := make(map[string]string)

	if _, err := cfg.Context(c.Name); err == nil {
		fields["name"] = fmt.Sprintf("a context called %q exists already", c.Name)
	} else if err := (config.Context{Name: c.Name, API: config.DefaultAPI}).Validate(); err != nil {
		fields["name"] = reason(err, "")
	}

	if err := (config.Context{Name: config.DefaultContextName, API: c.API}).Validate(); err != nil {
		fields["api"] = reason(err, "its api ")
	}

	if err := (config.Context{Name: config.DefaultContextName, API: config.DefaultAPI, Ingress: c.Ingress}).Validate(); err != nil {
		fields["ingress"] = reason(err, "its ingress ")
	}

	return fields
}

// reason is what an invalid context error says of the field it is about,
// whose name the error starts with after its kind.
func reason(err error, field string) string {
	said := strings.TrimPrefix(err.Error(), config.ErrInvalidContext.Error()+": ")

	return strings.TrimPrefix(said, field)
}
