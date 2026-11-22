package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

// errUnrenewable is a session that cannot be renewed however it is asked:
// it is over, as one the estate refuses to renew is.
var errUnrenewable = errors.New("the session cannot be renewed")

// tokenSource is the source of the access tokens of a context's session.
type tokenSource struct {
	manager *Manager
	context string
}

var _ api.TokenSource = &tokenSource{}

// identityField is the field a renewal refused for the account names.
const identityField = "identity"

// TokenSource is the source of the access tokens of a context's session.
// Its tokens are renewed before they run out and once the API refuses one;
// a context with no session, or whose session is over, is signed in to
// again in the browser when the manager may do that by itself, and is
// ErrNotLoggedIn when it may not.
func (m *Manager) TokenSource(contextName string) api.TokenSource {
	return &tokenSource{manager: m, context: contextName}
}

// Token is the session's access token, renewed first when it runs out
// within renewBefore.
func (s *tokenSource) Token(ctx context.Context) (string, error) {
	return s.manager.token(ctx, s.context, func(kept credentials.Credentials) bool {
		return !expiresWithin(kept.AccessToken, renewBefore)
	})
}

// Renew is a token to use once rejected was refused: the session's when it
// is another, which something else renewed it to meanwhile, and a renewed
// one when it is not.
func (s *tokenSource) Renew(ctx context.Context, rejected string) (string, error) {
	return s.manager.token(ctx, s.context, func(kept credentials.Credentials) bool {
		return kept.AccessToken != rejected && !expiresWithin(kept.AccessToken, renewBefore)
	})
}

// token is the access token of a context's session: the one kept, when
// usable says it may be used, and a renewed one when not. The session is
// remembered once it is read, and read again, once nothing else holds it,
// only before it is renewed: another process may have renewed it, or logged
// out of it, meanwhile.
func (m *Manager) token(ctx context.Context, contextName string, usable func(credentials.Credentials) bool) (string, error) {
	release, err := m.hold(ctx, contextName)
	if err != nil {
		return "", err
	}

	defer release()

	if kept, found := m.remembered(contextName); found && usable(kept) {
		return kept.AccessToken, nil
	}

	kept, err := m.store.Get(contextName)
	if errors.Is(err, credentials.ErrNotFound) {
		m.forget(contextName)

		return m.loginAgain(ctx, contextName, false)
	} else if err != nil {
		return "", err
	}

	m.remember(contextName, kept)

	if usable(kept) {
		return kept.AccessToken, nil
	}

	current, err := m.config.Context(contextName)
	if err != nil {
		return "", err
	}

	session, err := m.renew(ctx, current, kept)
	if refused(err) {
		if err := m.store.Delete(contextName); err != nil {
			return "", err
		}

		m.forget(contextName)

		if reason, refusedAccount := accountRefusal(err); refusedAccount {
			return "", &AccountRefusedError{Reason: reason}
		}

		return m.loginAgain(ctx, contextName, true)
	} else if err != nil {
		return "", fmt.Errorf("renewing the session of %q: %w", contextName, err)
	}

	if err := m.store.Set(contextName, session); err != nil {
		return "", err
	}

	m.remember(contextName, session)

	return session.AccessToken, nil
}

// loginAgain signs in to a context in the browser when the manager may do
// that by itself, saying why on the prompt, and is ErrNotLoggedIn when it
// may not. ended says whether there was a session, which is over.
func (m *Manager) loginAgain(ctx context.Context, contextName string, ended bool) (string, error) {
	if !m.interactive {
		if ended {
			return "", ErrSessionEnded
		}

		return "", ErrNotLoggedIn
	}

	current, err := m.config.Context(contextName)
	if err != nil {
		return "", err
	}

	if ended {
		m.say("The session of %q (%s) has ended: logging in again.\n", current.Name, current.API)
	} else {
		m.say("Not logged in to %q (%s): logging in.\n", current.Name, current.API)
	}

	session, err := m.loginWithBrowser(ctx, current)
	if err != nil {
		return "", err
	}

	return session.AccessToken, nil
}

// renew asks the estate for a session's next tokens: at the API's refresh
// endpoint for a session signed in to with a password, and at its token
// endpoint, by the application it was handed to, for one an authorization
// server handed over.
func (m *Manager) renew(ctx context.Context, current config.Context, kept credentials.Credentials) (credentials.Credentials, error) {
	switch kept.Method {
	case credentials.MethodPassword:
		return m.renewPassword(ctx, current, kept)
	case credentials.MethodOAuth:
		return m.renewOAuth(ctx, kept)
	default:
		return credentials.Credentials{}, fmt.Errorf("%w: it was kept as %q", errUnrenewable, kept.Method)
	}
}

// renewOAuth renews a session an authorization server handed over, at its
// token endpoint, as the application it was handed to.
func (m *Manager) renewOAuth(ctx context.Context, kept credentials.Credentials) (credentials.Credentials, error) {
	if len(kept.TokenEndpoint) == 0 || len(kept.ClientID) == 0 {
		return credentials.Credentials{}, fmt.Errorf("%w: it names no token endpoint or application", errUnrenewable)
	}

	var answer tokens

	err := m.postForm(ctx, kept.TokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {kept.RefreshToken},
		"client_id":     {kept.ClientID},
	}, &answer)
	if err != nil {
		return credentials.Credentials{}, err
	}

	return renewed(kept, answer)
}

// refused reports whether err is the estate refusing to renew a session,
// rather than failing to: the session is over.
func refused(err error) bool {
	var (
		apiRefusal   *api.Error
		oauthRefusal *oauthError
	)

	switch {
	case errors.Is(err, errUnrenewable):
		return true
	case errors.As(err, &apiRefusal):
		return refusal(apiRefusal.StatusCode)
	case errors.As(err, &oauthRefusal):
		return refusal(oauthRefusal.StatusCode)
	default:
		return false
	}
}

// accountRefusal is why the estate refused to renew a session for its
// account rather than for the session itself, as a password session's
// renewal names it: a blocked account, or one that is gone.
func accountRefusal(err error) (string, bool) {
	var apiRefusal *api.Error
	if !errors.As(err, &apiRefusal) {
		return "", false
	}

	reason, found := apiRefusal.Fields[identityField]

	return reason, found && len(reason) > 0
}

// refusal reports whether status is one a session's renewal is refused
// with.
func refusal(status int) bool {
	return status == http.StatusBadRequest || status == http.StatusUnauthorized
}
