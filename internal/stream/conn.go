package stream

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// Conn is a TCP connection to a port a VM publishes, carried by a websocket
// through the ingress. It is a net.Conn whose addresses are the websocket's,
// and a deadline that passes ends it, as it ends a websocket.
type Conn struct {
	socket *socket
	input  messageReader
}

var _ net.Conn = (*Conn)(nil)

// forward is what the forward action is asked with.
type forward struct {
	Port uint `json:"port"`
}

// Forward opens a connection to port of the VM uuid, one of the ports it
// publishes.
func (d *Dialer) Forward(ctx context.Context, vm string, port uint) (*Conn, error) {
	s, err := d.open(ctx, KindVMs, vm, ActionForward, forward{Port: port})
	if err != nil {
		return nil, err
	}

	c := &Conn{socket: s}
	c.input = messageReader{socket: s, ended: connEnded}

	return c, nil
}

// Read reads what the other end sent, and is io.EOF once it closed the
// connection.
func (c *Conn) Read(p []byte) (int, error) {
	return c.input.Read(p)
}

// Write sends p to the other end.
func (c *Conn) Write(p []byte) (int, error) {
	return c.socket.writeData(nil, p)
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.socket.close()
}

// LocalAddr is the local address of the websocket the connection is carried
// on.
func (c *Conn) LocalAddr() net.Addr {
	return c.socket.conn.LocalAddr()
}

// RemoteAddr is the address of the ingress the connection is carried
// through.
func (c *Conn) RemoteAddr() net.Addr {
	return c.socket.conn.RemoteAddr()
}

// SetDeadline sets when reading and writing give up.
func (c *Conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	return c.SetWriteDeadline(t)
}

// SetReadDeadline sets when reading gives up.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.socket.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets when writing gives up.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.socket.setWriteDeadline(t)
}

// connEnded is what a connection that stopped being read means: its end when
// the other end closed it, an unexpected end when it was lost, and an error
// saying why when it could not be dialled.
func connEnded(err error) error {
	if code, reason, closed := closeCode(err); closed {
		switch code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived:
			return io.EOF
		case websocket.CloseAbnormalClosure:
			return io.ErrUnexpectedEOF
		}

		return &CloseError{Code: code, Reason: reason}
	}

	return err
}
