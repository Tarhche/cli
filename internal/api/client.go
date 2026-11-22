// Package api is a client of the Tarhche API: it signs requests with the
// session it is given, renews a token the API refuses, and turns answers
// that are not successes into errors that say why.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout bounds a request the client makes with the HTTP client it
// builds itself.
const DefaultTimeout = 60 * time.Second

// TokenSource hands a client the access tokens it signs its requests with.
type TokenSource interface {
	// Token is a token to sign a request with.
	Token(ctx context.Context) (string, error)

	// Renew is a token to sign a request with once rejected was refused,
	// which a source renews its session for, or signs in again for when it
	// may.
	Renew(ctx context.Context, rejected string) (string, error)
}

// Client sends requests to the API.
type Client struct {
	base      *url.URL
	http      *http.Client
	tokens    TokenSource
	userAgent string
	language  string
}

// Option configures a client.
type Option func(*Client)

// WithHTTPClient sends requests through h.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithTokens signs requests with the tokens of source. A client without one
// sends them anonymously.
func WithTokens(source TokenSource) Option {
	return func(c *Client) { c.tokens = source }
}

// WithUserAgent introduces the client as userAgent.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) { c.userAgent = userAgent }
}

// LanguageHeader is the header the API reads the language to answer in
// from.
const LanguageHeader = "X-Language-Code"

// WithLanguage asks the API to answer in language, which is what the
// messages of its validation errors are written in.
func WithLanguage(language string) Option {
	return func(c *Client) { c.language = language }
}

// New is a client of the API at base, an absolute http(s) URL.
func New(base string, options ...Option) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || len(parsed.Host) == 0 || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%q is not an absolute http or https url", base)
	}

	c := &Client{base: parsed, http: &http.Client{Timeout: DefaultTimeout}}
	for _, option := range options {
		option(c)
	}

	return c, nil
}

// BaseURL is where the API is.
func (c *Client) BaseURL() *url.URL {
	u := *c.base

	return &u
}

// Tokens is the source the client signs its requests with, nil when it
// sends them anonymously.
func (c *Client) Tokens() TokenSource {
	return c.tokens
}

// URL is the absolute URL of path, with query.
func (c *Client) URL(path string, query url.Values) string {
	u := c.BaseURL()
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")

	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	return u.String()
}

// Get asks for path and decodes the answer into out.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.Do(ctx, http.MethodGet, path, query, nil, out)
}

// Post sends body to path and decodes the answer into out.
func (c *Client) Post(ctx context.Context, path string, body any, out any) error {
	return c.Do(ctx, http.MethodPost, path, nil, body, out)
}

// Put sends body to path and decodes the answer into out.
func (c *Client) Put(ctx context.Context, path string, body any, out any) error {
	return c.Do(ctx, http.MethodPut, path, nil, body, out)
}

// Patch sends body to path and decodes the answer into out.
func (c *Client) Patch(ctx context.Context, path string, body any, out any) error {
	return c.Do(ctx, http.MethodPatch, path, nil, body, out)
}

// Delete asks for path to be deleted and decodes the answer into out.
func (c *Client) Delete(ctx context.Context, path string, query url.Values, out any) error {
	return c.Do(ctx, http.MethodDelete, path, query, nil, out)
}

// Do sends a request and decodes the JSON it is answered with into out,
// which nil discards. A body is sent as JSON, but for bytes and a reader,
// which are sent as they are.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	content, contentType, err := encode(body)
	if err != nil {
		return err
	}

	request, err := c.NewRequest(ctx, method, path, query, content, contentType)
	if err != nil {
		return err
	}

	response, err := c.Send(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if out == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, response.Body)

		return nil
	}

	if raw, isRaw := out.(*json.RawMessage); isRaw {
		read, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("reading the answer to %s %s: %w", method, path, err)
		}

		*raw = read

		return nil
	}

	if err := json.NewDecoder(response.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("reading the answer to %s %s: %w", method, path, err)
	}

	return nil
}

// NewRequest is a request for path, with query, carrying body as
// contentType. A body that is to be sent again once a token is renewed has to
// be one http.NewRequest knows how to read again: bytes, a string or a
// buffer.
func (c *Client) NewRequest(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.URL(path, query), body)
	if err != nil {
		return nil, err
	}

	if len(contentType) > 0 {
		request.Header.Set("Content-Type", contentType)
	}

	request.Header.Set("Accept", "application/json")

	return request, nil
}

// Send signs request and sends it, once more with a renewed token when the
// first is refused. An answer that is not a success is an *Error, and its
// body is read; the body of a success is the caller's to close.
func (c *Client) Send(request *http.Request) (*http.Response, error) {
	if len(c.userAgent) > 0 {
		request.Header.Set("User-Agent", c.userAgent)
	}

	if len(c.language) > 0 {
		request.Header.Set(LanguageHeader, c.language)
	}

	token, err := c.token(request.Context())
	if err != nil {
		return nil, err
	}

	response, err := c.send(request, token)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusUnauthorized && c.tokens != nil && replayable(request) {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()

		renewed, err := c.tokens.Renew(request.Context(), token)
		if err != nil {
			return nil, err
		}

		retry := request.Clone(request.Context())
		if request.GetBody != nil {
			if retry.Body, err = request.GetBody(); err != nil {
				return nil, err
			}
		}

		if response, err = c.send(retry, renewed); err != nil {
			return nil, err
		}
	}

	if response.StatusCode >= http.StatusBadRequest {
		defer response.Body.Close()

		return nil, readError(request, response)
	}

	return response, nil
}

func (c *Client) token(ctx context.Context) (string, error) {
	if c.tokens == nil {
		return "", nil
	}

	return c.tokens.Token(ctx)
}

func (c *Client) send(request *http.Request, token string) (*http.Response, error) {
	if len(token) > 0 {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", request.Method, request.URL.Path, err)
	}

	return response, nil
}

func replayable(request *http.Request) bool {
	return request.Body == nil || request.Body == http.NoBody || request.GetBody != nil
}

func encode(body any) (io.Reader, string, error) {
	switch b := body.(type) {
	case nil:
		return nil, "", nil
	case []byte:
		return bytes.NewReader(b), "application/json", nil
	case json.RawMessage:
		return bytes.NewReader(b), "application/json", nil
	case io.Reader:
		return b, "application/json", nil
	}

	content, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("encoding the request: %w", err)
	}

	return bytes.NewReader(content), "application/json", nil
}
