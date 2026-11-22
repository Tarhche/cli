package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
	"github.com/Tarhche/cli/internal/version"
)

const (
	// ClientName is what the CLI registers itself with an authorization
	// server as, which the page asking the user to approve it shows.
	ClientName = "Tarhche CLI"

	// metadataPath is where an estate describes its authorization server
	// (RFC 8414).
	metadataPath = "/.well-known/oauth-authorization-server"

	// callbackPath is where on the loopback interface the browser is sent
	// back to.
	callbackPath = "/callback"

	// loginTimeout bounds how long a browser login waits for the user.
	loginTimeout = 5 * time.Minute

	// shutdownTimeout bounds how long the loopback server is given to
	// finish answering the browser once the login is over.
	shutdownTimeout = 2 * time.Second
)

// endpoints are where an authorization server is asked: for a code, for
// tokens, and to register.
type endpoints struct {
	authorization *url.URL
	token         string
	registration  string
}

// metadata is how an authorization server describes itself (RFC 8414).
type metadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

// registration is what the CLI says of itself as it registers (RFC 7591): a
// public client, which proves it asked for a code with PKCE rather than with
// a secret, since every copy of it would hold the same one.
type registration struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// callback is what the browser was sent back with: a code, or why there is
// none.
type callback struct {
	code string
	err  error
}

// LoginWithBrowser signs in to a context in a browser, through its
// authorization server, and keeps the session.
func (m *Manager) LoginWithBrowser(ctx context.Context, contextName string) error {
	release, err := m.hold(ctx, contextName)
	if err != nil {
		return err
	}

	defer release()

	current, err := m.config.Context(contextName)
	if err != nil {
		return err
	}

	_, err = m.loginWithBrowser(ctx, current)

	return err
}

// loginWithBrowser signs in to a context in a browser and keeps the session.
func (m *Manager) loginWithBrowser(ctx context.Context, current config.Context) (credentials.Credentials, error) {
	session, err := m.authorize(ctx, current)
	if err != nil {
		return credentials.Credentials{}, err
	}

	if err := m.store.Set(current.Name, session); err != nil {
		return credentials.Credentials{}, err
	}

	m.remember(current.Name, session)

	return session, nil
}

// authorize is the session an authorization server hands over once the user
// approved the CLI in their browser: the browser is sent back to the
// loopback interface with a code, which is collected with the verifier of
// the challenge it was asked for with.
func (m *Manager) authorize(ctx context.Context, current config.Context) (credentials.Credentials, error) {
	server, err := m.discover(ctx, current.API)
	if err != nil {
		return credentials.Credentials{}, err
	}

	listener, app, err := listen(current.OAuthClient)
	if err != nil {
		return credentials.Credentials{}, err
	}

	defer func() { _ = listener.Close() }()

	verifier, state := newVerifier(), rand.Text()

	if app != nil {
		known, err := m.knows(ctx, server.page(*app, verifier, state))
		if err != nil {
			return credentials.Credentials{}, err
		}

		if !known {
			app = nil
		}
	}

	if app == nil {
		registered, err := m.register(ctx, server, "http://"+listener.Addr().String()+callbackPath)
		if err != nil {
			return credentials.Credentials{}, err
		}

		if err := m.keepClient(current, registered); err != nil {
			return credentials.Credentials{}, err
		}

		app = &registered
	}

	code, err := m.approve(ctx, current, listener, *app, server.page(*app, verifier, state), state)
	if err != nil {
		return credentials.Credentials{}, err
	}

	var answer tokens

	err = m.postForm(ctx, server.token, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {app.RedirectURI},
		"client_id":     {app.ID},
		"code_verifier": {verifier},
	}, &answer)
	if err != nil {
		return credentials.Credentials{}, fmt.Errorf("collecting the session: %w", err)
	}

	if len(answer.AccessToken) == 0 {
		return credentials.Credentials{}, errNoToken
	}

	return credentials.Credentials{
		Method:        credentials.MethodOAuth,
		AccessToken:   answer.AccessToken,
		RefreshToken:  answer.RefreshToken,
		ClientID:      app.ID,
		TokenEndpoint: server.token,
	}, nil
}

// approve sends the user to page in their browser, and is the code it is
// sent back to the loopback interface with once they approved the CLI.
func (m *Manager) approve(ctx context.Context, current config.Context, listener net.Listener, app config.OAuthClient, page, state string) (string, error) {
	callbacks := make(chan callback, 1)

	server := &http.Server{Handler: answerCallback(pathOf(app.RedirectURI), state, callbacks), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		_ = server.Shutdown(ctx)
	}()

	m.say("Opening a browser to log in to %q (%s).\nIf it does not open, visit:\n\n    %s\n\n", current.Name, current.API, page)

	if err := m.browser(page); err != nil {
		m.say("The browser could not be opened: %s\n", err)
	}

	m.say("Waiting for the login in the browser...\n")

	timeout := time.NewTimer(m.loginTimeout)
	defer timeout.Stop()

	select {
	case got := <-callbacks:
		if got.err != nil {
			return "", fmt.Errorf("the login was refused: %w", got.err)
		}

		return got.code, nil
	case <-timeout.C:
		return "", fmt.Errorf("timed out after %s waiting for the login in the browser", m.loginTimeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// discover is where an estate's authorization server is asked, as it
// describes itself, and where the API serves it when it does not.
func (m *Manager) discover(ctx context.Context, api string) (endpoints, error) {
	base, err := absolute(strings.TrimRight(api, "/"))
	if err != nil {
		return endpoints{}, err
	}

	var described metadata

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.JoinPath(metadataPath).String(), nil)
	if err == nil && m.send(request, &described) != nil {
		described = metadata{}
	}

	if err := ctx.Err(); err != nil {
		return endpoints{}, err
	}

	authorization, err := absolute(described.AuthorizationEndpoint)
	if err != nil {
		authorization = base.JoinPath("/oauth/authorize")
	}

	return endpoints{
		authorization: authorization,
		token:         absoluteOr(described.TokenEndpoint, base.JoinPath("/oauth/token").String()),
		registration:  absoluteOr(described.RegistrationEndpoint, base.JoinPath("/oauth/register").String()),
	}, nil
}

// page is the authorization endpoint asked for a code for app, with the
// challenge of verifier and state, which the browser is sent to.
func (e endpoints) page(app config.OAuthClient, verifier, state string) string {
	asked := *e.authorization

	query := asked.Query()
	query.Set("response_type", "code")
	query.Set("client_id", app.ID)
	query.Set("redirect_uri", app.RedirectURI)
	query.Set("code_challenge", challengeOf(verifier))
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)

	asked.RawQuery = query.Encode()

	return asked.String()
}

// knows reports whether the authorization server still knows an
// application, by asking for page without going where it sends the
// browser: one nobody ever approved is thrown away a day after it
// registered, and a browser sent to its page would be shown that rather
// than sent back.
func (m *Manager) knows(ctx context.Context, page string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return false, err
	}

	request.Header.Set("User-Agent", version.UserAgent())

	asking := *m.http
	asking.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	response, err := asking.Do(request)
	if err != nil {
		return false, err
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusBadRequest {
		return true, nil
	}

	refusal := readOAuthError(response)
	if response.StatusCode == http.StatusUnauthorized || refusal.Code == "invalid_client" || refusal.Code == "invalid_redirect_uri" {
		return false, nil
	}

	return false, fmt.Errorf("the authorization server refused the login: %w", refusal)
}

// register registers the CLI with an authorization server, to be sent back
// to redirectURI.
func (m *Manager) register(ctx context.Context, server endpoints, redirectURI string) (config.OAuthClient, error) {
	body, err := json.Marshal(registration{
		ClientName:              ClientName,
		RedirectURIs:            []string{redirectURI},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	})
	if err != nil {
		return config.OAuthClient{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.registration, bytes.NewReader(body))
	if err != nil {
		return config.OAuthClient{}, err
	}

	request.Header.Set("Content-Type", "application/json")

	var registered struct {
		ClientID string `json:"client_id"`
	}

	if err := m.send(request, &registered); err != nil {
		return config.OAuthClient{}, fmt.Errorf("registering with the authorization server: %w", err)
	}

	if len(registered.ClientID) == 0 {
		return config.OAuthClient{}, errors.New("registering with the authorization server: it answered no client_id")
	}

	return config.OAuthClient{ID: registered.ClientID, RedirectURI: redirectURI}, nil
}

// keepClient keeps the application the CLI registered as the context's, in
// the configuration, so that it is registered once.
func (m *Manager) keepClient(current config.Context, app config.OAuthClient) error {
	m.configs.Lock()
	defer m.configs.Unlock()

	current.OAuthClient = &app

	if err := m.config.SetContext(current); err != nil {
		return err
	}

	return m.config.Save()
}

// listen listens on the loopback interface for the browser to be sent back:
// at the address of the application the context registered when it is
// free, so that it is registered once, and at a new one, which is to be
// registered, otherwise: an authorization server sends a browser back to an
// address an application registered, port and all.
func listen(saved *config.OAuthClient) (net.Listener, *config.OAuthClient, error) {
	if address, isLoopback := loopbackAddress(saved); isLoopback {
		if listener, err := net.Listen("tcp", address); err == nil {
			return listener, saved, nil
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("listening on the loopback interface for the browser: %w", err)
	}

	return listener, nil, nil
}

// loopbackAddress is the address on the loopback interface an application
// is sent back to, when it is one.
func loopbackAddress(app *config.OAuthClient) (string, bool) {
	if app == nil || len(app.ID) == 0 {
		return "", false
	}

	parsed, err := url.Parse(app.RedirectURI)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || len(parsed.Port()) == 0 {
		return "", false
	}

	return parsed.Host, true
}

// pathOf is the path of a redirect uri, which the browser is sent back to.
func pathOf(redirectURI string) string {
	parsed, err := url.Parse(redirectURI)
	if err != nil || len(parsed.Path) == 0 {
		return "/"
	}

	return parsed.Path
}

// answerCallback is where the browser is sent back to: it hands the code it
// was sent back with, or the reason it was not given one, to callbacks, and
// tells the user they may close the window. A request for another login
// than this one, one whose state is not this one's, is turned away.
func answerCallback(path, state string, callbacks chan<- callback) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(rw, r)

			return
		}

		query := r.URL.Query()
		if query.Get("state") != state {
			writePage(rw, http.StatusBadRequest, "This is not the login the CLI is waiting for", "Close this window, and use the page the CLI opened last.")

			return
		}

		var got callback

		switch {
		case len(query.Get("error")) > 0:
			got.err = &oauthError{Code: query.Get("error"), Description: query.Get("error_description")}
		case len(query.Get("code")) > 0:
			got.code = query.Get("code")
		default:
			writePage(rw, http.StatusBadRequest, "The browser came back without a code", "Close this window, and log in again.")

			return
		}

		select {
		case callbacks <- got:
		default:
			writePage(rw, http.StatusConflict, "This login is over already", "You can close this window.")

			return
		}

		if got.err != nil {
			writePage(rw, http.StatusOK, "The login failed", got.err.Error()+". You can close this window.")

			return
		}

		writePage(rw, http.StatusOK, "Logged in to Tarhche", "You can close this window, and go back to the terminal.")
	})
}

// pageTemplate is what the browser is shown once it is sent back.
var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · Tarhche CLI</title>
<style>
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; font-family: system-ui, sans-serif; background: #f8f9fa; color: #212529; }
main { max-width: 32rem; padding: 2rem; text-align: center; }
h1 { font-size: 1.5rem; }
</style>
</head>
<body>
<main>
<h1>{{.Title}}</h1>
<p>{{.Message}}</p>
</main>
</body>
</html>
`))

// writePage answers the browser with a page saying title and message.
func writePage(rw http.ResponseWriter, status int, title, message string) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-store")
	rw.WriteHeader(status)

	_ = pageTemplate.Execute(rw, struct{ Title, Message string }{Title: title, Message: message})
}

// absolute is raw, when it is an absolute http or https url.
func absolute(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || len(parsed.Host) == 0 || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%q is not an absolute http or https url", raw)
	}

	return parsed, nil
}

// absoluteOr is endpoint when it is an absolute http or https url, and
// fallback otherwise.
func absoluteOr(endpoint, fallback string) string {
	if _, err := absolute(endpoint); err != nil {
		return fallback
	}

	return endpoint
}
