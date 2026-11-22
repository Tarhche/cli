package stream

import (
	"context"
	"errors"
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

	"github.com/Tarhche/cli/internal/stream/streamtest"
)

// tokens is a token source that hands over token, and renewed once it is
// asked to renew.
type tokens struct {
	token    string
	renewed  string
	tokenErr error
	renewErr error

	lock     sync.Mutex
	rejected []string
}

func (s *tokens) Token(context.Context) (string, error) {
	return s.token, s.tokenErr
}

func (s *tokens) Renew(_ context.Context, rejected string) (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.rejected = append(s.rejected, rejected)

	return s.renewed, s.renewErr
}

func (s *tokens) Rejected() []string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return append([]string(nil), s.rejected...)
}

// serve is a dialer of a fake ingress serving ingress, signed with the token
// it takes.
func serve(t *testing.T, ingress *streamtest.Ingress, options ...Option) *Dialer {
	t.Helper()

	server := httptest.NewServer(ingress)
	t.Cleanup(server.Close)

	d, err := NewDialer(server.URL, &tokens{token: streamtest.Token}, options...)
	require.NoError(t, err)

	return d
}

func TestNewDialer(t *testing.T) {
	t.Parallel()

	t.Run("an http or a ws url is dialled as ws, an https or a wss one as wss", func(t *testing.T) {
		t.Parallel()

		cases := map[string]string{
			"http://ingress.example":   "ws://ingress.example/vms/u/attach",
			"ws://ingress.example/":    "ws://ingress.example/vms/u/attach",
			"https://ingress.example":  "wss://ingress.example/vms/u/attach",
			"wss://ingress.example":    "wss://ingress.example/vms/u/attach",
			"https://ingress.example/": "wss://ingress.example/vms/u/attach",
		}

		for base, want := range cases {
			d, err := NewDialer(base, nil)
			require.NoError(t, err, base)

			got, err := d.url(KindVMs, "u", "attach", nil)
			require.NoError(t, err)
			assert.Equal(t, want, got, base)
		}
	})

	t.Run("a url that is not absolute, or of another scheme, is an error", func(t *testing.T) {
		t.Parallel()

		for _, base := range []string{"", "ingress.example", "/ingress", "ftp://ingress.example", "://"} {
			_, err := NewDialer(base, nil)
			assert.Error(t, err, base)
		}
	})

	t.Run("options configure it", func(t *testing.T) {
		t.Parallel()

		d, err := NewDialer("https://ingress.example", nil, WithUserAgent("tarhche-cli/test"), WithPingInterval(5))

		require.NoError(t, err)
		assert.Equal(t, "tarhche-cli/test", d.header.Get("User-Agent"))
		assert.EqualValues(t, 5, d.ping)
	})
}

func TestDialerURL(t *testing.T) {
	t.Parallel()

	t.Run("the kind, the uuid and the action are escaped under the ingress's own path", func(t *testing.T) {
		t.Parallel()

		d, err := NewDialer("https://example.com/ingress/?ignored=1#fragment", nil)
		require.NoError(t, err)

		got, err := d.url("vms", "a b/c", "attach", nil)

		require.NoError(t, err)
		assert.Equal(t, "wss://example.com/ingress/vms/a%20b%2Fc/attach", got)
	})

	t.Run("a payload is its json in the query", func(t *testing.T) {
		t.Parallel()

		d, err := NewDialer("http://example.com", nil)
		require.NoError(t, err)

		got, err := d.url("vms", "u", "forward", forward{Port: 8080})
		require.NoError(t, err)

		parsed, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, `{"port":8080}`, parsed.Query().Get("payload"))
	})

	t.Run("a payload that cannot be encoded is an error", func(t *testing.T) {
		t.Parallel()

		d, err := NewDialer("http://example.com", nil)
		require.NoError(t, err)

		_, err = d.url("vms", "u", "exec", make(chan int))

		assert.Error(t, err)
	})
}

func TestDialerOpen(t *testing.T) {
	t.Parallel()

	accept := func(t *streamtest.Terminal) { _ = t.End() }

	t.Run("a stream is offered the token beside bearer, and introduced as the dialer is", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Attach: accept}
		d := serve(t, ingress, WithUserAgent("tarhche-cli/test"))

		terminal, err := d.Attach(context.Background(), KindVMs, "0190")
		require.NoError(t, err)
		require.NoError(t, terminal.Close())

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Equal(t, streamtest.Request{Kind: "vms", UUID: "0190", Action: "attach", Protocols: []string{"bearer", streamtest.Token}, UserAgent: "tarhche-cli/test"}, requests[0])
	})

	t.Run("a refused token is renewed once, and the stream asked for again", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Token: "renewed", Attach: accept}
		server := httptest.NewServer(ingress)
		t.Cleanup(server.Close)

		source := &tokens{token: "stale", renewed: "renewed"}
		d, err := NewDialer(server.URL, source)
		require.NoError(t, err)

		terminal, err := d.Attach(context.Background(), KindTasks, "0190")
		require.NoError(t, err)
		require.NoError(t, terminal.Close())

		assert.Equal(t, []string{"stale"}, source.Rejected())

		requests := ingress.Requests()
		require.Len(t, requests, 2)
		assert.Equal(t, []string{"bearer", "stale"}, requests[0].Protocols)
		assert.Equal(t, []string{"bearer", "renewed"}, requests[1].Protocols)
	})

	t.Run("a renewed token that is refused too is an error", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Token: "never", Attach: accept}
		server := httptest.NewServer(ingress)
		t.Cleanup(server.Close)

		d, err := NewDialer(server.URL, &tokens{token: "stale", renewed: "also-stale"})
		require.NoError(t, err)

		_, err = d.Attach(context.Background(), KindVMs, "0190")

		var refused *Error
		require.ErrorAs(t, err, &refused)
		assert.Equal(t, http.StatusUnauthorized, refused.StatusCode)
		assert.Len(t, ingress.Requests(), 2)
	})

	t.Run("a token that cannot be renewed is the error", func(t *testing.T) {
		t.Parallel()

		renewErr := errors.New("signed out")
		server := httptest.NewServer(&streamtest.Ingress{Token: "never", Attach: accept})
		t.Cleanup(server.Close)

		d, err := NewDialer(server.URL, &tokens{token: "stale", renewErr: renewErr})
		require.NoError(t, err)

		_, err = d.Attach(context.Background(), KindVMs, "0190")

		assert.ErrorIs(t, err, renewErr)
	})

	t.Run("a token that cannot be had is the error", func(t *testing.T) {
		t.Parallel()

		tokenErr := errors.New("not logged in")
		d, err := NewDialer("http://127.0.0.1:1", &tokens{tokenErr: tokenErr})
		require.NoError(t, err)

		_, err = d.Attach(context.Background(), KindVMs, "0190")

		assert.ErrorIs(t, err, tokenErr)
	})

	t.Run("without a token source no token is offered, and nothing is renewed", func(t *testing.T) {
		t.Parallel()

		ingress := &streamtest.Ingress{Attach: accept}
		server := httptest.NewServer(ingress)
		t.Cleanup(server.Close)

		d, err := NewDialer(server.URL, nil)
		require.NoError(t, err)

		_, err = d.Attach(context.Background(), KindVMs, "0190")

		var refused *Error
		require.ErrorAs(t, err, &refused)
		assert.Equal(t, http.StatusUnauthorized, refused.StatusCode)

		requests := ingress.Requests()
		require.Len(t, requests, 1)
		assert.Empty(t, requests[0].Protocols)
	})

	t.Run("an ingress that cannot be reached is an error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.NotFoundHandler())
		server.Close()

		d, err := NewDialer(server.URL, &tokens{token: streamtest.Token})
		require.NoError(t, err)

		_, err = d.Attach(context.Background(), KindVMs, "0190")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot open vms/0190/attach")
	})

	t.Run("a refusal says what the ingress said", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			status  int
			body    string
			check   func(error) bool
			fields  map[string]string
			message string
			says    string
		}{
			{status: http.StatusNotFound, body: "no such vm", check: IsNotFound, message: "no such vm", says: "cannot open vms/0190/attach: it is not there, or not yours (no such vm)"},
			{status: http.StatusServiceUnavailable, body: "not reachable: the vm is not running", check: IsUnavailable, message: "not reachable: the vm is not running", says: "cannot open vms/0190/attach: it cannot be reached now (not reachable: the vm is not running)"},
			{status: http.StatusBadRequest, body: `{"errors":{"port":"invalid_port","command":"required_field","env":"is not valid"}}`, fields: map[string]string{"port": "invalid_port", "command": "required_field", "env": "is not valid"}, says: "cannot open vms/0190/attach: the ingress refused what it was asked (command: required field; env: is not valid; port: invalid port)"},
			{status: http.StatusBadRequest, body: "invalid payload: the action is asked with nothing", message: "invalid payload: the action is asked with nothing", says: "cannot open vms/0190/attach: the ingress refused what it was asked (invalid payload: the action is asked with nothing)"},
			{status: http.StatusTooManyRequests, body: "", check: IsTooManyRequests, says: "cannot open vms/0190/attach: too many streams were opened from here; try again shortly"},
			{status: http.StatusBadGateway, body: "", says: "cannot open vms/0190/attach: the ingress answered 502 Bad Gateway"},
			{status: 599, body: "odd", message: "odd", says: "cannot open vms/0190/attach: the ingress answered 599 (odd)"},
		}

		for _, c := range cases {
			ingress := &streamtest.Ingress{Attach: accept, Refuse: func(streamtest.Request) (int, string) { return c.status, c.body }}
			d := serve(t, ingress)

			_, err := d.Attach(context.Background(), KindVMs, "0190")

			var refused *Error
			require.ErrorAs(t, err, &refused, c.says)
			assert.Equal(t, c.status, refused.StatusCode)
			assert.Equal(t, "vms/0190/attach", refused.Stream)
			assert.Equal(t, c.fields, refused.Fields)
			assert.Equal(t, c.message, refused.Message)
			assert.Equal(t, c.says, err.Error())

			if c.check != nil {
				assert.True(t, c.check(err), c.says)
			}
		}
	})
}

func TestError(t *testing.T) {
	t.Parallel()

	t.Run("each refusal is said in words of its own", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "cannot open vms/u/exec: the session's token was refused", (&Error{Stream: "vms/u/exec", StatusCode: http.StatusUnauthorized}).Error())
		assert.Equal(t, "cannot open vms/u/exec: it is not there, or not yours", (&Error{Stream: "vms/u/exec", StatusCode: http.StatusNotFound}).Error())
	})

	t.Run("a refusal for too many streams says when to ask again", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "cannot open vms/u/forward: too many streams were opened from here; try again in 12 seconds", (&Error{Stream: "vms/u/forward", StatusCode: http.StatusTooManyRequests, RetryAfter: "12"}).Error())
		assert.Equal(t, "cannot open vms/u/forward: too many streams were opened from here; try again after Wed, 21 Oct 2026 07:28:00 GMT", (&Error{Stream: "vms/u/forward", StatusCode: http.StatusTooManyRequests, RetryAfter: "Wed, 21 Oct 2026 07:28:00 GMT"}).Error())
	})

	t.Run("only refusals of their statuses are not found and unavailable", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsNotFound(errors.New("other")))
		assert.False(t, IsNotFound(&Error{StatusCode: http.StatusServiceUnavailable}))
		assert.False(t, IsUnavailable(&Error{StatusCode: http.StatusNotFound}))
		assert.True(t, IsUnavailable(&Error{StatusCode: http.StatusServiceUnavailable}))
		assert.False(t, IsTooManyRequests(&Error{StatusCode: http.StatusServiceUnavailable}))
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken") }

func TestRefusal(t *testing.T) {
	t.Parallel()

	t.Run("an answer without a body is its status alone", func(t *testing.T) {
		t.Parallel()

		err := refusal("vms/u/attach", &http.Response{StatusCode: http.StatusNotFound}, errors.New("bad handshake"))

		assert.Equal(t, &Error{Stream: "vms/u/attach", StatusCode: http.StatusNotFound}, err)
	})

	t.Run("how long to wait is what the answer's Retry-After says", func(t *testing.T) {
		t.Parallel()

		err := refusal("vms/u/forward", &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"30"}}}, errors.New("bad handshake"))

		assert.Equal(t, &Error{Stream: "vms/u/forward", StatusCode: http.StatusTooManyRequests, RetryAfter: "30"}, err)
	})

	t.Run("a body that cannot be read is left out", func(t *testing.T) {
		t.Parallel()

		err := refusal("vms/u/attach", &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(failingReader{})}, errors.New("bad handshake"))

		assert.Equal(t, &Error{Stream: "vms/u/attach", StatusCode: http.StatusNotFound}, err)
	})

	t.Run("a long body is read up to its bound", func(t *testing.T) {
		t.Parallel()

		body := strings.Repeat("x", 2*maxRefusal)

		err := refusal("vms/u/attach", &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(body))}, errors.New("bad handshake"))

		var refused *Error
		require.ErrorAs(t, err, &refused)
		assert.Len(t, refused.Message, maxRefusal)
	})
}

func TestCloseError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "the stream was closed: dial tcp: refused (1011)", (&CloseError{Code: 1011, Reason: "dial tcp: refused"}).Error())
	assert.Equal(t, "the stream was closed (4000)", (&CloseError{Code: 4000}).Error())
}

func TestExitError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "tar: x: Cannot stat", (&ExitError{Code: 2, Stderr: "tar: x: Cannot stat\n"}).Error())
	assert.Equal(t, "exit status 3", (&ExitError{Code: 3, Stderr: " \n"}).Error())
	assert.Equal(t, 3, (&ExitError{Code: 3}).ExitCode())
}

func TestKeepAlive(t *testing.T) {
	t.Parallel()

	t.Run("pinging stops once a ping cannot be sent", func(t *testing.T) {
		t.Parallel()

		pings := 0

		keepAlive(time.Millisecond, make(chan struct{}), func() error {
			pings++

			if pings == 3 {
				return errors.New("gone")
			}

			return nil
		})

		assert.Equal(t, 3, pings)
	})

	t.Run("pinging stops once the socket is closed", func(t *testing.T) {
		t.Parallel()

		done := make(chan struct{})
		close(done)

		keepAlive(time.Hour, done, func() error { return errors.New("never pinged") })
	})
}

func TestDialerOpenPayload(t *testing.T) {
	t.Parallel()

	d, err := NewDialer("http://127.0.0.1:1", &tokens{token: streamtest.Token})
	require.NoError(t, err)

	_, err = d.open(context.Background(), KindVMs, "0190", "exec", make(chan int))

	assert.Error(t, err)
}
