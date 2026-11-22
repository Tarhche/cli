package stream

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// controlWait bounds writing a ping or a close, so that a peer that
	// stopped reading cannot hold either.
	controlWait = time.Second

	// maxMessage is the most a message written carries; more is written as
	// several.
	maxMessage = 32 << 10
)

// socket is the websocket a stream is carried on: written by one writer at a
// time, pinged while it is open, and closed once.
type socket struct {
	conn      *websocket.Conn
	writeLock sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
}

func newSocket(conn *websocket.Conn, interval time.Duration) *socket {
	s := &socket{conn: conn, done: make(chan struct{})}

	go keepAlive(interval, s.done, s.ping)

	return s
}

// ping pings the other side.
func (s *socket) ping() error {
	return s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(controlWait))
}

// keepAlive pings every interval until done is closed or a ping cannot be
// sent, the connection being gone.
func keepAlive(interval time.Duration, done <-chan struct{}, ping func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := ping(); err != nil {
				return
			}
		}
	}
}

// write sends one message.
func (s *socket) write(messageType int, data []byte) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	if err := s.conn.WriteMessage(messageType, data); err != nil {
		return s.failure(err)
	}

	return nil
}

// writeData sends data as binary messages of maxMessage bytes at most, each
// led by prefix, and is how much of data was sent.
func (s *socket) writeData(prefix, data []byte) (int, error) {
	written := 0

	for len(data) > 0 {
		n := min(len(data), maxMessage)

		message := make([]byte, 0, len(prefix)+n)
		message = append(message, prefix...)
		message = append(message, data[:n]...)

		if err := s.write(websocket.BinaryMessage, message); err != nil {
			return written, err
		}

		written += n
		data = data[n:]
	}

	return written, nil
}

// setWriteDeadline sets when writing gives up, between writes.
func (s *socket) setWriteDeadline(t time.Time) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	return s.conn.SetWriteDeadline(t)
}

// next is the next message, of whatever type, and the reader of its data.
func (s *socket) next() (int, io.Reader, error) {
	messageType, message, err := s.conn.NextReader()
	if err != nil {
		return 0, nil, s.failure(err)
	}

	return messageType, message, nil
}

// failure is what an error reading or writing means: net.ErrClosed once the
// socket was closed on this side, else the error as it is.
func (s *socket) failure(err error) error {
	if s.closed.Load() {
		return net.ErrClosed
	}

	return err
}

// close says goodbye with 1000 and lets the connection go. Reads and writes
// blocked on it return.
func (s *socket) close() error {
	var err error

	s.closeOnce.Do(func() {
		s.closed.Store(true)
		close(s.done)

		_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(controlWait))

		err = s.conn.Close()
	})

	return err
}

// closeCode is the code and the reason the other side closed with, and
// whether err says it closed.
func closeCode(err error) (int, string, bool) {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return 0, "", false
	}

	return closeErr.Code, closeErr.Text, true
}

// messageReader reads the data of the binary messages of a socket as one
// stream of bytes, skipping text messages, and leaves what an error means to
// ended.
type messageReader struct {
	socket  *socket
	message io.Reader
	ended   func(error) error
}

// Read reads the next of the data, from the message being read or the next
// one.
func (r *messageReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		if r.message != nil {
			n, err := r.message.Read(p)
			if errors.Is(err, io.EOF) {
				r.message, err = nil, nil
			}

			if err != nil {
				return n, r.ended(r.socket.failure(err))
			}

			if n > 0 {
				return n, nil
			}

			continue
		}

		messageType, message, err := r.socket.next()
		if err != nil {
			return 0, r.ended(err)
		}

		if messageType == websocket.BinaryMessage {
			r.message = message
		}
	}
}
