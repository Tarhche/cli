package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingBody is an answer that cannot be read.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func (failingBody) Close() error { return nil }

func TestOAuthError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  oauthError
		said string
	}{
		{name: "a code and its description", err: oauthError{Code: "invalid_grant", Description: "the code is not valid"}, said: "invalid_grant: the code is not valid"},
		{name: "a code alone", err: oauthError{Code: "invalid_grant"}, said: "invalid_grant"},
		{name: "an answer that says neither", err: oauthError{StatusCode: http.StatusBadGateway, Body: "upstream down"}, said: "upstream down"},
		{name: "nothing at all", err: oauthError{StatusCode: http.StatusBadGateway}, said: "bad gateway"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.EqualError(t, &c.err, c.said)
		})
	}
}

func TestReadOAuthError(t *testing.T) {
	t.Parallel()

	answer := func(status int, body io.ReadCloser) *http.Response {
		return &http.Response{StatusCode: status, Body: body}
	}

	t.Run("what oauth says is read", func(t *testing.T) {
		t.Parallel()

		response := answer(http.StatusBadRequest, io.NopCloser(strings.NewReader(`{"error":"invalid_grant","error_description":"no"}`)))
		defer response.Body.Close()

		got := readOAuthError(response)

		assert.Equal(t, &oauthError{StatusCode: http.StatusBadRequest, Code: "invalid_grant", Description: "no"}, got)
	})

	t.Run("anything else is kept as it is", func(t *testing.T) {
		t.Parallel()

		response := answer(http.StatusNotFound, io.NopCloser(strings.NewReader(" 404 page not found\n")))
		defer response.Body.Close()

		got := readOAuthError(response)

		assert.Equal(t, &oauthError{StatusCode: http.StatusNotFound, Body: "404 page not found"}, got)
	})

	t.Run("json that is not oauth's is kept as it is", func(t *testing.T) {
		t.Parallel()

		response := answer(http.StatusBadRequest, io.NopCloser(strings.NewReader(`{"description":"no"}`)))
		defer response.Body.Close()

		got := readOAuthError(response)

		assert.Equal(t, &oauthError{StatusCode: http.StatusBadRequest, Body: `{"description":"no"}`}, got)
	})

	t.Run("an answer that cannot be read says its status", func(t *testing.T) {
		t.Parallel()

		response := answer(http.StatusTeapot, failingBody{})
		defer response.Body.Close()

		assert.EqualError(t, readOAuthError(response), "i'm a teapot")
	})
}

func TestManagerSend(t *testing.T) {
	t.Parallel()

	t.Run("a request is sent as the CLI, and its json answer read", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Contains(t, r.Header.Get("User-Agent"), "tarhche-cli/")
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			assert.NoError(t, r.ParseForm())
			assert.Equal(t, "b", r.PostForm.Get("a"))

			_, _ = io.WriteString(rw, `{"access_token":"t"}`)
		}))
		t.Cleanup(server.Close)

		var answer tokens
		require.NoError(t, New(nil, nil).postForm(context.Background(), server.URL, url.Values{"a": {"b"}}, &answer))

		assert.Equal(t, "t", answer.AccessToken)
	})

	t.Run("an endpoint that is no url is an error", func(t *testing.T) {
		t.Parallel()

		var answer tokens

		assert.Error(t, New(nil, nil).postForm(context.Background(), "http://%zz", nil, &answer))
	})

	t.Run("an endpoint that cannot be reached is an error", func(t *testing.T) {
		t.Parallel()

		var answer tokens

		assert.Error(t, New(nil, nil).postForm(context.Background(), "http://127.0.0.1:1/token", nil, &answer))
	})
}

func TestVerifier(t *testing.T) {
	t.Parallel()

	verifier := newVerifier()

	assert.Regexp(t, `^[A-Za-z0-9._~-]{43,128}$`, verifier, "what RFC 7636 allows")
	assert.NotEqual(t, verifier, newVerifier())

	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), challengeOf(verifier))
}
