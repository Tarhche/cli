// Package streamtest is a workload ingress for tests: it opens the streams
// of docs/streams.md for the token it is given, checks what they are asked
// with, and serves each action as the test says. It speaks the protocols on
// its own, sharing nothing with the client, so that a test holds the client
// to the document rather than to itself.
package streamtest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Token is the access token streams are opened with unless an Ingress says
// otherwise.
const Token = "test-token"

// writeWait bounds a control message the ingress writes.
const writeWait = time.Second

// Request is a stream that was asked for.
type Request struct {
	Kind   string
	UUID   string
	Action string

	// Payload is the payload's JSON, empty when it was asked with none.
	Payload string

	// Protocols are the subprotocols offered.
	Protocols []string

	UserAgent string
}

// Ingress serves streams: it refuses a request whose subprotocols are not
// "bearer" and its token with 401, one its Refuse refuses as it says, and
// one for an action it has no handler for with 404, and upgrades the rest.
type Ingress struct {
	// Token is the token a stream has to be offered; Token when empty.
	Token string

	// Attach, Exec and Forward serve the actions of those names.
	Attach  func(*Terminal)
	Exec    func(*Exec)
	Forward func(*Raw)

	// Refuse answers a request with a status and a body instead of
	// upgrading it, when the status it is it returns is not 0. A body that
	// is a JSON object is answered as JSON.
	Refuse func(Request) (int, string)

	lock     sync.Mutex
	requests []Request
}

var _ http.Handler = (*Ingress)(nil)

// Requests are the streams asked for so far, refused ones among them.
func (i *Ingress) Requests() []Request {
	i.lock.Lock()
	defer i.lock.Unlock()

	return append([]Request(nil), i.requests...)
}

// ServeHTTP opens a stream, or refuses it.
func (i *Ingress) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) != 3 {
		http.Error(w, "no such stream", http.StatusNotFound)

		return
	}

	request := Request{
		Kind:      segments[0],
		UUID:      segments[1],
		Action:    segments[2],
		Payload:   r.URL.Query().Get("payload"),
		Protocols: websocket.Subprotocols(r),
		UserAgent: r.UserAgent(),
	}

	i.lock.Lock()
	i.requests = append(i.requests, request)
	i.lock.Unlock()

	if !i.authorized(request) {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	if i.Refuse != nil {
		if status, body := i.Refuse(request); status != 0 {
			refuse(w, status, body)

			return
		}
	}

	serve := i.handler(request)
	if serve == nil {
		http.Error(w, "no such stream", http.StatusNotFound)

		return
	}

	upgrader := websocket.Upgrader{
		Subprotocols: []string{"bearer"},
		CheckOrigin:  func(*http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	defer func() { _ = conn.Close() }()

	serve(newSession(conn, request))
}

func (i *Ingress) authorized(request Request) bool {
	token := i.Token
	if len(token) == 0 {
		token = Token
	}

	return len(request.Protocols) == 2 && request.Protocols[0] == "bearer" && request.Protocols[1] == token
}

func (i *Ingress) handler(request Request) func(*session) {
	switch {
	case request.Action == "attach" && i.Attach != nil:
		return func(s *session) { i.Attach(&Terminal{session: s}) }
	case request.Action == "exec" && i.Exec != nil:
		return func(s *session) { serveExec(s, i.Exec) }
	case request.Action == "forward" && i.Forward != nil:
		return func(s *session) { serveRaw(s, i.Forward) }
	}

	return nil
}

func refuse(w http.ResponseWriter, status int, body string) {
	if strings.HasPrefix(body, "{") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)

		return
	}

	http.Error(w, body, status)
}

// session is an upgraded stream, written by one writer at a time.
type session struct {
	Request Request

	conn      *websocket.Conn
	writeLock sync.Mutex
	pings     atomic.Int32
}

func newSession(conn *websocket.Conn, request Request) *session {
	s := &session{Request: request, conn: conn}

	conn.SetPingHandler(func(data string) error {
		s.pings.Add(1)

		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(writeWait))
	})

	return s
}

// Pings is how many pings the client sent so far, which are counted as the
// stream is read.
func (s *session) Pings() int {
	return int(s.pings.Load())
}

// Ping pings the client, which answers once it reads.
func (s *session) Ping() error {
	return s.conn.WriteControl(websocket.PingMessage, []byte("ingress"), time.Now().Add(writeWait))
}

func (s *session) write(messageType int, data []byte) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	return s.conn.WriteMessage(messageType, data)
}

// CloseWith closes the websocket with code and reason, and lets the
// connection go.
func (s *session) CloseWith(code int, reason string) error {
	err := s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(writeWait))

	_ = s.conn.Close()

	return err
}

// Frame writes raw, a websocket frame or a part of one, to the connection as
// it is: a message cut short, say, before it is dropped.
func (s *session) Frame(raw []byte) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	_, err := s.conn.UnderlyingConn().Write(raw)

	return err
}

// Drop lets the connection go without closing the websocket, as a server
// that went away does.
func (s *session) Drop() error {
	return s.conn.Close()
}

// Terminal is a terminal stream, served as the test says.
type Terminal struct {
	*session
}

// Message is something a client said on a terminal: what it typed, or a
// control message.
type Message struct {
	// Input is what was typed, the data of a binary message.
	Input []byte

	// Control is a text message, decoded.
	Control *Control
}

// Control is a terminal's control message.
type Control struct {
	Type string `json:"type"`
	Rows uint   `json:"rows"`
	Cols uint   `json:"cols"`
}

// Next is the next thing the client said, and an error once it went away.
func (t *Terminal) Next() (Message, error) {
	for {
		messageType, data, err := t.conn.ReadMessage()
		if err != nil {
			return Message{}, err
		}

		if messageType == websocket.BinaryMessage {
			return Message{Input: data}, nil
		}

		var control Control
		if json.Unmarshal(data, &control) == nil {
			return Message{Control: &control}, nil
		}
	}
}

// Output writes p as what the terminal shows.
func (t *Terminal) Output(p []byte) error {
	return t.write(websocket.BinaryMessage, p)
}

// Text writes a text message, which a terminal's client ignores.
func (t *Terminal) Text(text string) error {
	return t.write(websocket.TextMessage, []byte(text))
}

// End closes the terminal as a shell that ended does.
func (t *Terminal) End() error {
	return t.CloseWith(websocket.CloseNormalClosure, "")
}
