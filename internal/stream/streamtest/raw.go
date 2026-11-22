package streamtest

import (
	"encoding/json"
	"errors"
	"io"
	"net"

	"github.com/gorilla/websocket"
)

// Raw is a forwarded connection's stream, served as the test says: what
// the client sends is read from it, and what is written to it is sent to
// the client.
type Raw struct {
	*session

	// Port is the port the forward was asked for.
	Port uint

	message io.Reader
}

// serveRaw reads a forward's payload, and serves it with serve.
func serveRaw(s *session, serve func(*Raw)) {
	var payload struct {
		Port uint `json:"port"`
	}

	if err := json.Unmarshal([]byte(s.Request.Payload), &payload); err != nil {
		_ = s.CloseWith(websocket.CloseProtocolError, "the payload is not json")

		return
	}

	serve(&Raw{session: s, Port: payload.Port})
}

// Read reads what the client sent, and is io.EOF once it closed the
// websocket, however it did.
func (r *Raw) Read(p []byte) (int, error) {
	for {
		if r.message != nil {
			n, err := r.message.Read(p)
			if errors.Is(err, io.EOF) {
				r.message, err = nil, nil
			}

			if n > 0 || err != nil {
				return n, err
			}

			continue
		}

		messageType, message, err := r.conn.NextReader()
		if err != nil {
			return 0, io.EOF
		}

		if messageType == websocket.BinaryMessage {
			r.message = message
		}
	}
}

// Write sends p to the client.
func (r *Raw) Write(p []byte) (int, error) {
	if err := r.write(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close closes the websocket with 1000, as the other end of the connection
// closing it does.
func (r *Raw) Close() error {
	return r.CloseWith(websocket.CloseNormalClosure, "")
}

// Fail closes the websocket with 1011 and reason, as a port that could not
// be dialled does.
func (r *Raw) Fail(reason string) error {
	return r.CloseWith(websocket.CloseInternalServerErr, reason)
}

// Proxy serves a forward by connecting it to address: the bytes go both
// ways until either end closes, and an address that cannot be dialled fails
// the stream, saying why.
func Proxy(r *Raw, address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		_ = r.Fail(err.Error())

		return
	}

	defer func() { _ = conn.Close() }()

	go func() {
		_, _ = io.Copy(conn, r)
		_ = conn.Close()
	}()

	_, _ = io.Copy(r, conn)
	_ = r.Close()
}
