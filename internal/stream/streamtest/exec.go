package streamtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

// ErrGone is what a command's input reads once its client went away before
// ending it.
var ErrGone = errors.New("the client went away")

// ExecPayload is what the exec action is asked with.
type ExecPayload struct {
	Command    []string `json:"command"`
	TTY        bool     `json:"tty"`
	Rows       uint     `json:"rows"`
	Cols       uint     `json:"cols"`
	Env        []string `json:"env"`
	WorkingDir string   `json:"working_dir"`
}

// Size is a size a client said its terminal is.
type Size struct {
	Rows uint `json:"rows"`
	Cols uint `json:"cols"`
}

// Exec is a command stream, served as the test says: the client's messages
// are read as they come, whatever the test does meanwhile.
type Exec struct {
	*session

	// Payload is what the command was asked with.
	Payload ExecPayload

	// Stdin is what the client writes to the command: it ends when the
	// client ends it, and fails with ErrGone when the client goes away.
	Stdin io.Reader

	// Stdout and Stderr write what the command says on them.
	Stdout io.Writer
	Stderr io.Writer

	// Resizes are the sizes the client says, in order.
	Resizes <-chan Size

	// Done is closed once the client went away.
	Done <-chan struct{}

	stdinEnded atomic.Bool
}

// serveExec reads a command's payload and its client's messages, and serves
// it with serve.
func serveExec(s *session, serve func(*Exec)) {
	var payload ExecPayload
	if err := json.Unmarshal([]byte(s.Request.Payload), &payload); err != nil {
		_ = s.CloseWith(websocket.CloseProtocolError, "the payload is not json")

		return
	}

	stdin := newBuffer()
	resizes := make(chan Size, 64)
	done := make(chan struct{})

	e := &Exec{
		session: s,
		Payload: payload,
		Stdin:   stdin,
		Stdout:  channelWriter{session: s, channel: 1},
		Stderr:  channelWriter{session: s, channel: 2},
		Resizes: resizes,
		Done:    done,
	}

	go func() {
		defer close(done)
		defer stdin.closeWithError(ErrGone)

		for {
			messageType, data, err := s.conn.ReadMessage()
			if err != nil {
				return
			}

			if messageType != websocket.BinaryMessage || len(data) == 0 {
				continue
			}

			switch data[0] {
			case 0:
				stdin.write(data[1:])
			case 4:
				var size Size
				if json.Unmarshal(data[1:], &size) == nil {
					resizes <- size
				}
			case 5:
				e.stdinEnded.Store(true)
				stdin.closeWithError(io.EOF)
			}
		}
	}()

	serve(e)
}

// StdinEnded reports whether the client ended the command's input.
func (e *Exec) StdinEnded() bool {
	return e.stdinEnded.Load()
}

// Exit says the command exited with code, and closes the websocket with
// 1000.
func (e *Exec) Exit(code int) error {
	return e.exit(fmt.Sprintf(`{"exit_code":%d}`, code))
}

// Fail says the command could not be waited for, and why, and closes the
// websocket with 1000.
func (e *Exec) Fail(reason string) error {
	content, err := json.Marshal(map[string]any{"exit_code": -1, "error": reason})
	if err != nil {
		return err
	}

	return e.exit(string(content))
}

func (e *Exec) exit(status string) error {
	if err := e.Send(3, []byte(status)); err != nil {
		return err
	}

	return e.CloseWith(websocket.CloseNormalClosure, "")
}

// Send writes data on channel, as it is.
func (e *Exec) Send(channel byte, data []byte) error {
	return e.write(websocket.BinaryMessage, append([]byte{channel}, data...))
}

// Raw writes a message of messageType, data and all, as it is.
func (e *Exec) Raw(messageType int, data []byte) error {
	return e.write(messageType, data)
}

// channelWriter writes what it is given on a channel.
type channelWriter struct {
	session *session
	channel byte
}

func (w channelWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if err := w.session.write(websocket.BinaryMessage, append([]byte{w.channel}, p...)); err != nil {
		return 0, err
	}

	return len(p), nil
}

// buffer is a pipe that never blocks its writer: what is written waits in
// it until it is read.
type buffer struct {
	lock  sync.Mutex
	ready *sync.Cond
	data  []byte
	err   error
}

func newBuffer() *buffer {
	b := &buffer{}
	b.ready = sync.NewCond(&b.lock)

	return b
}

func (b *buffer) write(p []byte) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.err == nil {
		b.data = append(b.data, p...)
		b.ready.Broadcast()
	}
}

func (b *buffer) closeWithError(err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.err == nil {
		b.err = err
		b.ready.Broadcast()
	}
}

// Read reads what was written, waiting for it, and the error it was closed
// with once all of it was read.
func (b *buffer) Read(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for len(b.data) == 0 && b.err == nil {
		b.ready.Wait()
	}

	if len(b.data) == 0 {
		return 0, b.err
	}

	n := copy(p, b.data)
	b.data = b.data[n:]

	return n, nil
}

// Run serves a command by running it on this machine, its standard streams
// the stream's, and says the status it exited with. A command that cannot
// be started exits with 127, saying why on its standard error.
func Run(e *Exec) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-e.Done:
			cancel()
		case <-ctx.Done():
		}
	}()

	command := exec.CommandContext(ctx, e.Payload.Command[0], e.Payload.Command[1:]...)
	command.Dir = e.Payload.WorkingDir
	command.Env = append(os.Environ(), e.Payload.Env...)
	command.Stdin = e.Stdin
	command.Stdout = e.Stdout
	command.Stderr = e.Stderr

	err := command.Run()

	var exitErr *exec.ExitError

	switch {
	case err == nil:
		_ = e.Exit(0)
	case errors.As(err, &exitErr):
		_ = e.Exit(exitErr.ExitCode())
	default:
		_, _ = fmt.Fprintln(e.Stderr, err)
		_ = e.Exit(127)
	}
}

// Relay serves a command as nc does, the address it dials being address
// whatever the command names: the client's input is sent to it, and its end
// closes the connection's writing half; what the other end sends is the
// command's output, and the command exits with 0 once it closed the
// connection. An address that cannot be dialled exits with 1, saying why.
func Relay(e *Exec, address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		_, _ = fmt.Fprintf(e.Stderr, "nc: can't connect to remote host (%s): %s\n", address, err)
		_ = e.Exit(1)

		return
	}

	defer func() { _ = conn.Close() }()

	go func() {
		if _, err := io.Copy(conn, e.Stdin); err == nil {
			_ = conn.(*net.TCPConn).CloseWrite()
		} else {
			_ = conn.Close()
		}
	}()

	_, _ = io.Copy(e.Stdout, conn)
	_ = e.Exit(0)
}
