package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/gorilla/websocket"
)

// Terminal is a terminal in a workload with a shell on it: what is written
// to it is typed into it, and what is read from it is what it shows.
//
// Read from one goroutine at a time; Write and Resize may be called from
// others, and Close from any.
type Terminal struct {
	socket *socket
	output messageReader
}

// Attach opens a terminal on the resource uuid of kind, KindVMs or
// KindTasks.
func (d *Dialer) Attach(ctx context.Context, kind, uuid string) (*Terminal, error) {
	s, err := d.open(ctx, kind, uuid, "attach", nil)
	if err != nil {
		return nil, err
	}

	t := &Terminal{socket: s}
	t.output = messageReader{socket: s, ended: terminalEnded}

	return t, nil
}

// Read reads what the terminal shows, and is io.EOF once its shell ended.
func (t *Terminal) Read(p []byte) (int, error) {
	return t.output.Read(p)
}

// Write types p into the terminal.
func (t *Terminal) Write(p []byte) (int, error) {
	return t.socket.writeData(nil, p)
}

// Resize tells the terminal it is rows by cols; a size with no rows or no
// columns says nothing.
func (t *Terminal) Resize(rows, cols uint) error {
	if rows == 0 || cols == 0 {
		return nil
	}

	return t.socket.write(websocket.TextMessage, fmt.Appendf(nil, `{"type":"resize","rows":%d,"cols":%d}`, rows, cols))
}

// Close lets go of the terminal, which ends its shell.
func (t *Terminal) Close() error {
	return t.socket.close()
}

// terminalEnded is what a terminal that stopped being read means: its end
// when its shell ended, however the connection was let go of, and an error
// when the other side closed it for another reason.
func terminalEnded(err error) error {
	if code, reason, closed := closeCode(err); closed {
		switch code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure:
			return io.EOF
		}

		return &CloseError{Code: code, Reason: reason}
	}

	if errors.Is(err, net.ErrClosed) {
		return io.EOF
	}

	return err
}
