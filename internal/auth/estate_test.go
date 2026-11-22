package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/config"
	"github.com/Tarhche/cli/internal/credentials"
)

// failure is what a route of an estate answers instead of what it would.
type failure struct {
	status int
	body   string
}

// grant is what an authorization code was handed out for.
type grant struct {
	clientID    string
	redirectURI string
	challenge   string
}

// refreshGrant is the session a refresh token renews, and the application
// it was handed to, which is none for one signed in to with a password.
type refreshGrant struct {
	subject  string
	session  string
	clientID string
}

// endedSession is a session somebody ended, and the token they ended it
// with.
type endedSession struct {
	session string
	token   string
}

// estate is an estate of Tarhche a test serves: its API's login, refresh and
// sessions, and its authorization server, at the paths its metadata names
// (under /as) and at the API's own (under /oauth), as a user who approves
// every application would answer it.
type estate struct {
	*httptest.Server

	mux *http.ServeMux

	lock sync.Mutex

	passwords map[string]string
	clients   map[string]string
	codes     map[string]grant
	refreshes map[string]refreshGrant

	calls         map[string]int
	registrations []registration
	logins        []map[string]any
	ended         []endedSession
	issued        int
	sessions      int

	// what the estate is told to answer otherwise.
	noMetadata     bool
	metadata       map[string]string
	deny           bool
	noClientID     bool
	noAccessToken  bool
	noRefreshToken bool
	accessLifetime time.Duration
	fail           map[string]failure
}

// newEstate is an estate configured by options before it serves anything.
func newEstate(t *testing.T, options ...func(*estate)) *estate {
	t.Helper()

	e := &estate{
		mux:            http.NewServeMux(),
		passwords:      map[string]string{"jane": "secret"},
		clients:        make(map[string]string),
		codes:          make(map[string]grant),
		refreshes:      make(map[string]refreshGrant),
		calls:          make(map[string]int),
		metadata:       make(map[string]string),
		fail:           make(map[string]failure),
		accessLifetime: 3 * time.Minute,
	}

	for _, option := range options {
		option(e)
	}

	e.mux.HandleFunc("GET /.well-known/oauth-authorization-server", e.describe)

	for _, prefix := range []string{"/as", "/oauth"} {
		e.mux.HandleFunc("POST "+prefix+"/register", e.register)
		e.mux.HandleFunc("GET "+prefix+"/authorize", e.authorize)
		e.mux.HandleFunc("POST "+prefix+"/token", e.token)
	}

	e.mux.HandleFunc("POST /api/auth/login", e.login)
	e.mux.HandleFunc("POST /api/auth/token/refresh", e.refresh)
	e.mux.HandleFunc("DELETE /api/dashboard/profile/sessions/{uuid}", e.endSession)

	e.Server = httptest.NewServer(e)
	t.Cleanup(e.Close)

	return e
}

func (e *estate) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route := r.Method + " " + r.URL.Path

	e.lock.Lock()
	e.calls[route]++
	failing, fails := e.fail[route]
	e.lock.Unlock()

	if fails {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(failing.status)
		_, _ = io.WriteString(rw, failing.body)

		return
	}

	e.mux.ServeHTTP(rw, r)
}

// called is how many times a route was asked.
func (e *estate) called(route string) int {
	e.lock.Lock()
	defer e.lock.Unlock()

	return e.calls[route]
}

// set changes how the estate answers while it serves.
func (e *estate) set(change func(*estate)) {
	e.lock.Lock()
	defer e.lock.Unlock()

	change(e)
}

// knowClient registers an application the way it would have registered
// itself.
func (e *estate) knowClient(id, redirectURI string) {
	e.set(func(e *estate) { e.clients[id] = redirectURI })
}

// startSession hands over a session as a login would have, for an
// application, or for none when it was signed in to with a password.
func (e *estate) startSession(clientID string, lifetime time.Duration) tokens {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.sessions++

	return e.issue("user-1", fmt.Sprintf("session-%d", e.sessions), clientID, lifetime)
}

// issue hands over tokens of a session; its caller holds the lock.
func (e *estate) issue(subject, session, clientID string, lifetime time.Duration) tokens {
	e.issued++

	issued := tokens{
		AccessToken:  token(map[string]any{"sub": subject, "sid": session, "exp": time.Now().Add(lifetime).Unix(), "n": e.issued}),
		RefreshToken: token(map[string]any{"sub": subject, "sid": session, "exp": time.Now().Add(48 * time.Hour).Unix(), "n": e.issued, "aud": "refresh"}),
	}

	e.refreshes[issued.RefreshToken] = refreshGrant{subject: subject, session: session, clientID: clientID}

	return issued
}

// answer is what the estate answers a session over as, as it was told to.
func (e *estate) answer(issued tokens) map[string]any {
	answered := map[string]any{"access_token": issued.AccessToken, "refresh_token": issued.RefreshToken, "token_type": "Bearer", "expires_in": 180}

	if e.noAccessToken {
		delete(answered, "access_token")
	}

	if e.noRefreshToken {
		delete(answered, "refresh_token")
	}

	return answered
}

func (e *estate) describe(rw http.ResponseWriter, r *http.Request) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.noMetadata {
		http.NotFound(rw, r)

		return
	}

	described := map[string]string{
		"issuer":                 e.URL,
		"authorization_endpoint": e.URL + "/as/authorize",
		"token_endpoint":         e.URL + "/as/token",
		"registration_endpoint":  e.URL + "/as/register",
	}
	maps.Copy(described, e.metadata)

	writeJSON(rw, http.StatusOK, described)
}

func (e *estate) register(rw http.ResponseWriter, r *http.Request) {
	var asked registration
	if err := json.NewDecoder(r.Body).Decode(&asked); err != nil || len(asked.RedirectURIs) != 1 {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})

		return
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	e.registrations = append(e.registrations, asked)

	id := fmt.Sprintf("client-%d", len(e.registrations))
	if e.noClientID {
		id = ""
	}

	e.clients[id] = asked.RedirectURIs[0]

	writeJSON(rw, http.StatusCreated, map[string]any{"client_id": id, "redirect_uris": asked.RedirectURIs})
}

func (e *estate) authorize(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	e.lock.Lock()
	defer e.lock.Unlock()

	redirectURI, known := e.clients[query.Get("client_id")]
	if !known {
		writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "invalid_client", "error_description": "this application is not registered here"})

		return
	}

	if query.Get("redirect_uri") != redirectURI {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})

		return
	}

	back, err := url.Parse(redirectURI)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)

		return
	}

	values := url.Values{"state": {query.Get("state")}}

	switch {
	case query.Get("response_type") != "code" || query.Get("code_challenge_method") != "S256" || len(query.Get("code_challenge")) == 0:
		values.Set("error", "invalid_request")
	case e.deny:
		values.Set("error", "access_denied")
		values.Set("error_description", "the request was refused")
	default:
		e.issued++
		code := fmt.Sprintf("code-%d", e.issued)
		e.codes[code] = grant{clientID: query.Get("client_id"), redirectURI: redirectURI, challenge: query.Get("code_challenge")}
		values.Set("code", code)
	}

	back.RawQuery = values.Encode()

	http.Redirect(rw, r, back.String(), http.StatusFound)
}

func (e *estate) token(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_request"})

		return
	}

	form := r.PostForm
	refused := map[string]string{"error": "invalid_grant", "error_description": "the refresh token is not valid any more"}

	e.lock.Lock()
	defer e.lock.Unlock()

	switch form.Get("grant_type") {
	case "authorization_code":
		given, found := e.codes[form.Get("code")]
		delete(e.codes, form.Get("code"))

		sum := sha256.Sum256([]byte(form.Get("code_verifier")))
		if !found || given.clientID != form.Get("client_id") || given.redirectURI != form.Get("redirect_uri") || base64.RawURLEncoding.EncodeToString(sum[:]) != given.challenge {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_grant", "error_description": "the code is not valid"})

			return
		}

		e.sessions++
		writeJSON(rw, http.StatusOK, e.answer(e.issue("user-1", fmt.Sprintf("session-%d", e.sessions), given.clientID, e.accessLifetime)))
	case "refresh_token":
		kept, found := e.refreshes[form.Get("refresh_token")]
		if !found || len(kept.clientID) == 0 || kept.clientID != form.Get("client_id") {
			writeJSON(rw, http.StatusBadRequest, refused)

			return
		}

		writeJSON(rw, http.StatusOK, e.answer(e.issue(kept.subject, kept.session, kept.clientID, e.accessLifetime)))
	default:
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
	}
}

func (e *estate) login(rw http.ResponseWriter, r *http.Request) {
	var asked map[string]any
	if err := json.NewDecoder(r.Body).Decode(&asked); err != nil {
		rw.WriteHeader(http.StatusBadRequest)

		return
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	e.logins = append(e.logins, asked)

	identity, _ := asked["identity"].(string)
	password, _ := asked["password"].(string)

	if want, found := e.passwords[identity]; !found || want != password {
		writeJSON(rw, http.StatusBadRequest, map[string]any{"errors": map[string]string{"identity": "invalid identity or password"}})

		return
	}

	e.sessions++
	writeJSON(rw, http.StatusOK, e.answer(e.issue(identity, fmt.Sprintf("session-%d", e.sessions), "", e.accessLifetime)))
}

func (e *estate) refresh(rw http.ResponseWriter, r *http.Request) {
	var asked struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&asked); err != nil {
		rw.WriteHeader(http.StatusBadRequest)

		return
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	kept, found := e.refreshes[asked.Token]
	if !found || len(kept.clientID) > 0 {
		writeJSON(rw, http.StatusBadRequest, map[string]any{"errors": map[string]string{"token": "invalid value"}})

		return
	}

	writeJSON(rw, http.StatusOK, e.answer(e.issue(kept.subject, kept.session, "", e.accessLifetime)))
}

func (e *estate) endSession(rw http.ResponseWriter, r *http.Request) {
	e.lock.Lock()
	e.ended = append(e.ended, endedSession{session: r.PathValue("uuid"), token: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")})
	e.lock.Unlock()

	rw.WriteHeader(http.StatusNoContent)
}

// endedSessions are the sessions somebody ended, in order.
func (e *estate) endedSessions() []endedSession {
	e.lock.Lock()
	defer e.lock.Unlock()

	return append([]endedSession(nil), e.ended...)
}

// loginRequests are what the estate was asked to sign in with, in order.
func (e *estate) loginRequests() []map[string]any {
	e.lock.Lock()
	defer e.lock.Unlock()

	return append([]map[string]any(nil), e.logins...)
}

// registered are the registrations the estate took, in order.
func (e *estate) registered() []registration {
	e.lock.Lock()
	defer e.lock.Unlock()

	return append([]registration(nil), e.registrations...)
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(body)
}

// token is an unsigned JWT saying claims, which is all the CLI reads of
// one.
func token(claims map[string]any) string {
	payload, _ := json.Marshal(claims)

	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

// expiring is a token of session that runs out in lifetime.
func expiring(session string, lifetime time.Duration) string {
	return token(map[string]any{"sub": "user-1", "sid": session, "exp": time.Now().Add(lifetime).Unix()})
}

// follow is a browser that goes where it is sent, and wherever it is sent
// on, as a user who approves the CLI would.
func follow(page string) error {
	response, err := http.Get(page)
	if err != nil {
		return err
	}

	defer func() { _ = response.Body.Close() }()

	_, err = io.Copy(io.Discard, response.Body)

	return err
}

// harness is a manager of the context "test" of a configuration of its
// own, whose API is an estate's, keeping its sessions in store and saying
// what it does on prompt.
type harness struct {
	*Manager

	config *config.File
	prompt *bytes.Buffer
	store  credentials.Store
}

func newHarness(t *testing.T, e *estate, store credentials.Store, options ...Option) *harness {
	t.Helper()

	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, cfg.SetContext(config.Context{Name: "test", API: e.URL}))

	prompt := &bytes.Buffer{}

	defaults := []Option{WithHTTPClient(e.Client()), WithPrompt(prompt), WithBrowser(follow)}

	return &harness{Manager: New(cfg, store, append(defaults, options...)...), config: cfg, prompt: prompt, store: store}
}

// stored is the session kept for the context "test".
func (h *harness) stored(t *testing.T) credentials.Credentials {
	t.Helper()

	kept, err := h.store.Get("test")
	require.NoError(t, err)

	return kept
}

// canceled is a context that is over.
func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}
