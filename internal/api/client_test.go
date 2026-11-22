package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tokens struct {
	lock     sync.Mutex
	current  string
	renewed  string
	err      error
	renewErr error
	rejected []string
}

func (t *tokens) Token(context.Context) (string, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.current, t.err
}

func (t *tokens) Renew(_ context.Context, rejected string) (string, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.rejected = append(t.rejected, rejected)
	if t.renewErr != nil {
		return "", t.renewErr
	}

	t.current = t.renewed

	return t.current, nil
}

func serve(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("an absolute url is the base, without its trailing slash", func(t *testing.T) {
		t.Parallel()

		c, err := New("https://backend.example/prefix/")

		require.NoError(t, err)
		assert.Equal(t, "https://backend.example/prefix", c.BaseURL().String())
		assert.Nil(t, c.Tokens())
	})

	for _, base := range []string{"", "backend.example", "/api", "ftp://backend.example", "http://%zz"} {
		t.Run("refuses "+base, func(t *testing.T) {
			t.Parallel()

			_, err := New(base)

			assert.Error(t, err)
		})
	}

	t.Run("options are applied", func(t *testing.T) {
		t.Parallel()

		source := &tokens{}
		h := &http.Client{}

		c, err := New("https://backend.example", WithTokens(source), WithHTTPClient(h), WithUserAgent("agent"), WithLanguage("fa"))

		require.NoError(t, err)
		assert.Same(t, source, c.Tokens())
		assert.Same(t, h, c.http)
		assert.Equal(t, "agent", c.userAgent)
		assert.Equal(t, "fa", c.language)
	})
}

func TestClientWithTimeout(t *testing.T) {
	t.Parallel()

	source := &tokens{current: "token"}
	h := &http.Client{Timeout: time.Minute}

	c, err := New("https://backend.example", WithTokens(source), WithHTTPClient(h), WithUserAgent("agent"), WithLanguage("fa"))
	require.NoError(t, err)

	patient := c.WithTimeout(15 * time.Minute)

	assert.Equal(t, 15*time.Minute, patient.http.Timeout)
	assert.Equal(t, time.Minute, c.http.Timeout, "the client it was copied from is left as it was")
	assert.Same(t, source, patient.Tokens())
	assert.Equal(t, "agent", patient.userAgent)
	assert.Equal(t, "fa", patient.language)
	assert.Equal(t, c.BaseURL(), patient.BaseURL())
	assert.Zero(t, c.WithTimeout(0).http.Timeout)
}

func TestClientURL(t *testing.T) {
	t.Parallel()

	c, err := New("https://backend.example/prefix")
	require.NoError(t, err)

	assert.Equal(t, "https://backend.example/prefix/api/articles", c.URL("/api/articles", nil))
	assert.Equal(t, "https://backend.example/prefix/api/articles?page=2", c.URL("api/articles", url.Values{"page": {"2"}}))
}

func TestClientDo(t *testing.T) {
	t.Parallel()

	t.Run("a json body is sent and a json answer decoded", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/things", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			assert.Equal(t, "agent", r.Header.Get("User-Agent"))
			assert.Equal(t, "fa", r.Header.Get(LanguageHeader))

			var body map[string]string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, map[string]string{"name": "x"}, body)

			_, _ = io.WriteString(rw, `{"uuid":"1"}`)
		})

		c, err := New(server.URL, WithTokens(&tokens{current: "token"}), WithUserAgent("agent"), WithLanguage("fa"))
		require.NoError(t, err)

		var out map[string]string
		require.NoError(t, c.Post(context.Background(), "/api/things", map[string]string{"name": "x"}, &out))

		assert.Equal(t, map[string]string{"uuid": "1"}, out)
	})

	t.Run("each verb is sent as itself, with its query", func(t *testing.T) {
		t.Parallel()

		var (
			lock  sync.Mutex
			asked []string
		)

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			lock.Lock()
			asked = append(asked, r.Method+" "+r.URL.RequestURI())
			lock.Unlock()

			_, _ = io.WriteString(rw, `{}`)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		ctx := context.Background()
		require.NoError(t, c.Get(ctx, "/a", url.Values{"page": {"1"}}, nil))
		require.NoError(t, c.Put(ctx, "/b", map[string]int{}, nil))
		require.NoError(t, c.Patch(ctx, "/c", map[string]int{}, nil))
		require.NoError(t, c.Delete(ctx, "/d", url.Values{"cascade": {"true"}}, nil))

		assert.Equal(t, []string{"GET /a?page=1", "PUT /b", "PATCH /c", "DELETE /d?cascade=true"}, asked)
	})

	t.Run("bytes, raw json and readers are sent as they are", func(t *testing.T) {
		t.Parallel()

		var (
			lock   sync.Mutex
			bodies []string
		)

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			content, _ := io.ReadAll(r.Body)

			lock.Lock()
			bodies = append(bodies, string(content))
			lock.Unlock()
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		ctx := context.Background()
		require.NoError(t, c.Post(ctx, "/", []byte(`{"a":1}`), nil))
		require.NoError(t, c.Post(ctx, "/", json.RawMessage(`{"b":2}`), nil))
		require.NoError(t, c.Post(ctx, "/", strings.NewReader(`{"c":3}`), nil))

		assert.Equal(t, []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}, bodies)
	})

	t.Run("a body that cannot be encoded is an error", func(t *testing.T) {
		t.Parallel()

		c, err := New("https://backend.example")
		require.NoError(t, err)

		assert.ErrorContains(t, c.Post(context.Background(), "/", make(chan int), nil), "encoding")
	})

	t.Run("a raw message takes the answer as it is", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(rw, `{"items": [1, 2]}`)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		var raw json.RawMessage
		require.NoError(t, c.Get(context.Background(), "/", nil, &raw))

		assert.JSONEq(t, `{"items":[1,2]}`, string(raw))
	})

	t.Run("an empty answer leaves out as it was", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/none" {
				rw.WriteHeader(http.StatusNoContent)
			}
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		out := map[string]string{"kept": "yes"}
		require.NoError(t, c.Get(context.Background(), "/none", nil, &out))
		require.NoError(t, c.Get(context.Background(), "/empty", nil, &out))

		assert.Equal(t, map[string]string{"kept": "yes"}, out)
	})

	t.Run("an answer that is not json is an error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(rw, `not json`)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		var out map[string]any
		assert.ErrorContains(t, c.Get(context.Background(), "/things", nil, &out), "reading the answer to GET /things")
	})

	t.Run("a failure is an error saying what the api said", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(rw, `{"code":"not_found","message":"no vm \"x\""}`)
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		err = c.Get(context.Background(), "/vms/x", nil, nil)

		var apiErr *Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
		assert.Equal(t, http.MethodGet, apiErr.Method)
		assert.Equal(t, "/vms/x", apiErr.Path)
		assert.Equal(t, `no vm "x"`, err.Error())
		assert.True(t, IsNotFound(err))
	})

	t.Run("a request that cannot be sent is an error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(http.ResponseWriter, *http.Request) {})
		server.Close()

		c, err := New(server.URL)
		require.NoError(t, err)

		assert.ErrorContains(t, c.Get(context.Background(), "/gone", nil, nil), "GET /gone")
	})

	t.Run("a request that cannot be made is an error", func(t *testing.T) {
		t.Parallel()

		c, err := New("https://backend.example")
		require.NoError(t, err)

		assert.Error(t, c.Do(context.Background(), "BAD METHOD", "/", nil, nil, nil))
	})
}

func TestClientSend(t *testing.T) {
	t.Parallel()

	t.Run("a refused token is renewed and the request sent again, body and all", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			content, _ := io.ReadAll(r.Body)
			assert.Equal(t, `{"name":"x"}`, string(content))

			if r.Header.Get("Authorization") != "Bearer fresh" {
				rw.WriteHeader(http.StatusUnauthorized)

				return
			}

			_, _ = io.WriteString(rw, `{"ok":true}`)
		})

		source := &tokens{current: "stale", renewed: "fresh"}

		c, err := New(server.URL, WithTokens(source))
		require.NoError(t, err)

		var out map[string]bool
		require.NoError(t, c.Post(context.Background(), "/", map[string]string{"name": "x"}, &out))

		assert.Equal(t, map[string]bool{"ok": true}, out)
		assert.Equal(t, []string{"stale"}, source.rejected)
	})

	t.Run("a token refused twice is an unauthorized error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		})

		c, err := New(server.URL, WithTokens(&tokens{current: "stale", renewed: "also-stale"}))
		require.NoError(t, err)

		err = c.Get(context.Background(), "/", nil, nil)

		assert.True(t, IsUnauthorized(err))
		assert.Equal(t, "unauthorized", err.Error())
	})

	t.Run("a token that cannot be renewed is that error", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		})

		gone := errors.New("session gone")

		c, err := New(server.URL, WithTokens(&tokens{current: "stale", renewErr: gone}))
		require.NoError(t, err)

		assert.ErrorIs(t, c.Get(context.Background(), "/", nil, nil), gone)
	})

	t.Run("a token that cannot be had is that error", func(t *testing.T) {
		t.Parallel()

		missing := errors.New("not logged in")

		c, err := New("https://backend.example", WithTokens(&tokens{err: missing}))
		require.NoError(t, err)

		assert.ErrorIs(t, c.Get(context.Background(), "/", nil, nil), missing)
	})

	t.Run("a body that cannot be read again is not sent again", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		})

		source := &tokens{current: "stale", renewed: "fresh"}

		c, err := New(server.URL, WithTokens(source))
		require.NoError(t, err)

		request, err := c.NewRequest(context.Background(), http.MethodPost, "/", nil, io.NopCloser(strings.NewReader("x")), "text/plain")
		require.NoError(t, err)

		response, err := c.Send(request)
		if err == nil {
			_ = response.Body.Close()
		}

		assert.True(t, IsUnauthorized(err))
		assert.Empty(t, source.rejected)
	})

	t.Run("the body of a success is the caller's", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
			assert.Empty(t, r.Header.Get("Authorization"))

			_, _ = io.WriteString(rw, "file content")
		})

		c, err := New(server.URL)
		require.NoError(t, err)

		request, err := c.NewRequest(context.Background(), http.MethodPut, "/files", nil, strings.NewReader("x"), "text/plain")
		require.NoError(t, err)

		response, err := c.Send(request)
		require.NoError(t, err)

		defer response.Body.Close()

		content, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		assert.Equal(t, "file content", string(content))
	})
}

type explainer struct {
	lock      sync.Mutex
	explained []*Error
	client    **Client
	probed    error
}

func (e *explainer) Explain(ctx context.Context, refused *Error) error {
	e.lock.Lock()
	e.explained = append(e.explained, refused)
	e.lock.Unlock()

	e.probed = (*e.client).Get(Unexplained(ctx), "/probe", nil, nil)

	return fmt.Errorf("explained: %w", refused)
}

func TestClientExplain(t *testing.T) {
	t.Parallel()

	t.Run("a refusal is explained, and what the explainer asks is answered as it is", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusForbidden)
		})

		var client *Client
		explaining := &explainer{client: &client}

		client, err := New(server.URL, WithExplainer(explaining))
		require.NoError(t, err)

		err = client.Get(context.Background(), "/things", nil, nil)

		assert.EqualError(t, err, "explained: forbidden")
		assert.True(t, IsForbidden(err), "an explained refusal is still a refusal")
		require.Len(t, explaining.explained, 1)
		assert.Equal(t, "/things", explaining.explained[0].Path)
		assert.True(t, IsForbidden(explaining.probed), "the probe's refusal was not explained again")
	})

	t.Run("what is not a refusal is not explained", func(t *testing.T) {
		t.Parallel()

		server := serve(t, func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		})

		var client *Client
		explaining := &explainer{client: &client}

		client, err := New(server.URL, WithExplainer(explaining))
		require.NoError(t, err)

		assert.True(t, IsNotFound(client.Get(context.Background(), "/things", nil, nil)))
		assert.Empty(t, explaining.explained)
	})
}
