// Package stream opens the streams of docs/streams.md on a workload ingress:
// a terminal (attach), a command (exec) and a TCP connection to a port
// (forward), each a type of its own that the CLI and the desktop app use
// alike, whatever shows them.
package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Tarhche/cli/internal/api"
)

const (
	// KindVMs is the kind a VM's streams are opened on, as the plural the
	// ingress names it by.
	KindVMs = "vms"

	// KindTasks is the kind a code runner's task's streams are opened on.
	KindTasks = "tasks"

	// DefaultPingInterval is how often an open stream is pinged, so that
	// nothing on the way closes it for being idle.
	DefaultPingInterval = 30 * time.Second

	// bearerProtocol is the subprotocol a token is offered beside.
	bearerProtocol = "bearer"

	// handshakeTimeout bounds opening a stream, the upgrade included.
	handshakeTimeout = 45 * time.Second
)

// Dialer opens streams on a workload ingress, signed with a session's
// access tokens.
type Dialer struct {
	ingress   *url.URL
	tokens    api.TokenSource
	header    http.Header
	websocket websocket.Dialer
	ping      time.Duration
}

// Option configures a Dialer.
type Option func(*Dialer)

// WithUserAgent introduces the dialer as userAgent.
func WithUserAgent(userAgent string) Option {
	return func(d *Dialer) { d.header.Set("User-Agent", userAgent) }
}

// WithPingInterval pings an open stream every interval instead of every
// DefaultPingInterval.
func WithPingInterval(interval time.Duration) Option {
	return func(d *Dialer) { d.ping = interval }
}

// NewDialer is a dialer of the ingress at base, an absolute http(s) or ws(s)
// URL, signing streams with the tokens of source; a nil source opens them
// anonymously.
func NewDialer(base string, tokens api.TokenSource, options ...Option) (*Dialer, error) {
	ingress, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || len(ingress.Host) == 0 {
		return nil, fmt.Errorf("%q is not an absolute url of a workload ingress", base)
	}

	switch ingress.Scheme {
	case "http", "ws":
		ingress.Scheme = "ws"
	case "https", "wss":
		ingress.Scheme = "wss"
	default:
		return nil, fmt.Errorf("%q is not an http, https, ws or wss url", base)
	}

	d := &Dialer{
		ingress: ingress,
		tokens:  tokens,
		header:  http.Header{},
		ping:    DefaultPingInterval,
		websocket: websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: handshakeTimeout,
		},
	}

	for _, option := range options {
		option(d)
	}

	return d, nil
}

// open opens the stream action of the resource uuid of kind, asked with
// payload, which nil leaves out. A token the ingress refuses is renewed once.
func (d *Dialer) open(ctx context.Context, kind, uuid, action string, payload any) (*socket, error) {
	name := kind + "/" + uuid + "/" + action

	target, err := d.url(kind, uuid, action, payload)
	if err != nil {
		return nil, err
	}

	token, err := d.token(ctx)
	if err != nil {
		return nil, err
	}

	conn, response, err := d.handshake(ctx, target, token)
	if response != nil && response.StatusCode == http.StatusUnauthorized && d.tokens != nil {
		_ = response.Body.Close()

		renewed, renewErr := d.tokens.Renew(ctx, token)
		if renewErr != nil {
			return nil, renewErr
		}

		conn, response, err = d.handshake(ctx, target, renewed)
	}

	if err != nil {
		return nil, refusal(name, response, err)
	}

	_ = response.Body.Close()

	return newSocket(conn, d.ping), nil
}

// url is where the stream action of the resource uuid of kind is opened,
// its payload's JSON in the query.
func (d *Dialer) url(kind, uuid, action string, payload any) (string, error) {
	target := *d.ingress
	target.RawQuery = ""
	target.Fragment = ""
	target.Path = strings.TrimRight(d.ingress.Path, "/") + "/" + kind + "/" + uuid + "/" + action
	target.RawPath = strings.TrimRight(d.ingress.EscapedPath(), "/") + "/" + url.PathEscape(kind) + "/" + url.PathEscape(uuid) + "/" + url.PathEscape(action)

	if payload != nil {
		content, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("encoding the payload of %s: %w", action, err)
		}

		target.RawQuery = url.Values{"payload": {string(content)}}.Encode()
	}

	return target.String(), nil
}

func (d *Dialer) token(ctx context.Context) (string, error) {
	if d.tokens == nil {
		return "", nil
	}

	return d.tokens.Token(ctx)
}

// handshake upgrades a request for target to a websocket, the token offered
// as the subprotocols "bearer" and the token itself.
func (d *Dialer) handshake(ctx context.Context, target, token string) (*websocket.Conn, *http.Response, error) {
	dialer := d.websocket
	if len(token) > 0 {
		dialer.Subprotocols = []string{bearerProtocol, token}
	}

	return dialer.DialContext(ctx, target, d.header.Clone())
}
