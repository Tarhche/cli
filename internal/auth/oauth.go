package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Tarhche/cli/internal/version"
)

// maxAnswer bounds how much of what an authorization server answers is read.
const maxAnswer = 1 << 20

// tokens are what an estate hands a session over as: its own login and
// refresh endpoints and OAuth's token endpoint alike.
type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// oauthError is what an authorization server answered that is not a
// success, as OAuth says it: a code and a description.
type oauthError struct {
	StatusCode  int    `json:"-"`
	Code        string `json:"error"`
	Description string `json:"error_description"`

	// Body is what was answered when it says none of that.
	Body string `json:"-"`
}

func (e *oauthError) Error() string {
	switch {
	case len(e.Code) > 0 && len(e.Description) > 0:
		return e.Code + ": " + e.Description
	case len(e.Code) > 0:
		return e.Code
	case len(e.Body) > 0:
		return e.Body
	default:
		return strings.ToLower(http.StatusText(e.StatusCode))
	}
}

// readOAuthError reads what an authorization server answered that is not a
// success.
func readOAuthError(response *http.Response) *oauthError {
	refusal := &oauthError{StatusCode: response.StatusCode}

	content, err := io.ReadAll(io.LimitReader(response.Body, maxAnswer))
	if err != nil || json.Unmarshal(content, refusal) != nil || len(refusal.Code) == 0 {
		refusal.Code, refusal.Description = "", ""
		refusal.Body = strings.TrimSpace(string(content))
	}

	return refusal
}

// postForm posts values as a form to an authorization server's endpoint and
// decodes the JSON it answers into out.
func (m *Manager) postForm(ctx context.Context, endpoint string, values url.Values, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return m.send(request, out)
}

// send sends a request to an authorization server and decodes the JSON it
// answers into out; an answer that is not a success is an *oauthError.
func (m *Manager) send(request *http.Request, out any) error {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", version.UserAgent())

	response, err := m.http.Do(request)
	if err != nil {
		return err
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode >= http.StatusBadRequest {
		return readOAuthError(response)
	}

	if err := json.NewDecoder(io.LimitReader(response.Body, maxAnswer)).Decode(out); err != nil {
		return fmt.Errorf("reading the answer of %s: %w", request.URL.Redacted(), err)
	}

	return nil
}

// newVerifier is a PKCE code verifier: 256 random bits, in characters RFC
// 7636 allows.
func newVerifier() string {
	return rand.Text() + rand.Text()
}

// challengeOf is the S256 challenge of a code verifier.
func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}
