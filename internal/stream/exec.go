package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

// The channels of the channel protocol: the first byte of every message.
const (
	channelStdin  byte = 0
	channelStdout byte = 1
	channelStderr byte = 2
	channelExit   byte = 3
	channelResize byte = 4
	channelClose  byte = 5
)

// maxExit bounds how much of a command's exit is read.
const maxExit = 64 << 10

// ErrNoCommand is a command asked for with nothing to run.
var ErrNoCommand = errors.New("a command is required")

// ExecOptions are a command to run in a VM, and how.
type ExecOptions struct {
	// Command is the program and its arguments; it is required.
	Command []string

	// TTY runs the command on a terminal of Rows by Cols, where all it says
	// is on its standard output.
	TTY  bool
	Rows uint
	Cols uint

	// Env are its environment variables, each "NAME=value".
	Env []string

	// WorkingDir is the directory it runs in.
	WorkingDir string

	// Stderr is where what it says on its standard error is written, as it
	// is read; nil discards it.
	Stderr io.Writer
}

// execPayload is what the exec action is asked with.
type execPayload struct {
	Command    []string `json:"command"`
	TTY        bool     `json:"tty,omitempty"`
	Rows       uint     `json:"rows,omitempty"`
	Cols       uint     `json:"cols,omitempty"`
	Env        []string `json:"env,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`
}

// exit is how a command ended, as the server says it.
type exit struct {
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error"`
}

// Exec is a command running in a VM: what is written to it is its standard
// input, what is read from it its standard output, and Wait is how it
// exited.
//
// Read and Wait from one goroutine at a time; Write, CloseWrite and Resize
// may be called from others, and Close from any.
type Exec struct {
	socket     *socket
	stderr     io.Writer
	stdout     io.Reader
	exit       *exit
	err        error
	closeInput sync.Once
	inputErr   error
}

// Exec runs a command in the VM uuid.
func (d *Dialer) Exec(ctx context.Context, vm string, options ExecOptions) (*Exec, error) {
	if len(options.Command) == 0 || len(options.Command[0]) == 0 {
		return nil, ErrNoCommand
	}

	payload := execPayload{
		Command:    options.Command,
		TTY:        options.TTY,
		Env:        options.Env,
		WorkingDir: options.WorkingDir,
	}

	if options.TTY {
		payload.Rows, payload.Cols = options.Rows, options.Cols
	}

	s, err := d.open(ctx, KindVMs, vm, "exec", payload)
	if err != nil {
		return nil, err
	}

	stderr := options.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	return &Exec{socket: s, stderr: stderr}, nil
}

// Read reads the command's standard output, writing what it says on its
// standard error to its Stderr as it comes, and is io.EOF once it exited.
func (e *Exec) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		if e.stdout != nil {
			n, err := e.stdout.Read(p)
			if errors.Is(err, io.EOF) {
				e.stdout, err = nil, nil
			}

			if err != nil {
				return n, e.fail(e.socket.failure(err))
			}

			if n > 0 {
				return n, nil
			}

			continue
		}

		switch {
		case e.exit != nil:
			return 0, io.EOF
		case e.err != nil:
			return 0, e.err
		}

		if err := e.receive(); err != nil {
			return 0, err
		}
	}
}

// receive reads the next message: its standard output to be read, what it
// says on its standard error, or its exit.
func (e *Exec) receive() error {
	messageType, message, err := e.socket.next()
	if err != nil {
		return e.fail(err)
	}

	if messageType != websocket.BinaryMessage {
		return nil
	}

	var channel [1]byte
	if _, err := io.ReadFull(message, channel[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return e.fail(e.socket.failure(err))
	}

	switch channel[0] {
	case channelStdout:
		e.stdout = message
	case channelStderr:
		if _, err := io.Copy(e.stderr, message); err != nil {
			return e.fail(err)
		}
	case channelExit:
		var status exit
		if err := json.NewDecoder(io.LimitReader(message, maxExit)).Decode(&status); err != nil {
			return e.fail(fmt.Errorf("reading how the command exited: %w", err))
		}

		e.exit = &status
	}

	return nil
}

// fail keeps err as why the command's output stopped short.
func (e *Exec) fail(err error) error {
	if code, reason, closed := closeCode(err); closed {
		if code == websocket.CloseNormalClosure || code == websocket.CloseNoStatusReceived || code == websocket.CloseAbnormalClosure {
			err = errors.New("the stream ended before the command's exit was said")
		} else {
			err = &CloseError{Code: code, Reason: reason}
		}
	}

	e.err = err

	return err
}

// Wait reads what is left of the command's output, discarding it, until it
// exits, and is the status it exited with: -1 for one killed by a signal. A
// command that could not be waited for, or whose stream ended before saying
// how it exited, is an error.
func (e *Exec) Wait() (int, error) {
	if _, err := io.Copy(io.Discard, e); err != nil {
		return -1, err
	}

	if len(e.exit.Error) > 0 {
		return e.exit.ExitCode, fmt.Errorf("the command could not be waited for: %s", e.exit.Error)
	}

	return e.exit.ExitCode, nil
}

// Write writes p to the command's standard input.
func (e *Exec) Write(p []byte) (int, error) {
	return e.socket.writeData([]byte{channelStdin}, p)
}

// CloseWrite ends the command's standard input; it reads its end once it
// read what was written before. It is said once, however often it is
// called.
func (e *Exec) CloseWrite() error {
	e.closeInput.Do(func() {
		e.inputErr = e.socket.write(websocket.BinaryMessage, []byte{channelClose})
	})

	return e.inputErr
}

// Resize tells the command's terminal it is rows by cols; a size with no
// rows or no columns says nothing.
func (e *Exec) Resize(rows, cols uint) error {
	if rows == 0 || cols == 0 {
		return nil
	}

	message := fmt.Appendf([]byte{channelResize}, `{"rows":%d,"cols":%d}`, rows, cols)

	return e.socket.write(websocket.BinaryMessage, message)
}

// Close lets go of the command, which ends it if it is still running.
func (e *Exec) Close() error {
	return e.socket.close()
}

// Runner runs commands in a VM to their end.
type Runner struct {
	dialer *Dialer
	vm     string
}

// maxStderr bounds how much of what a command says on its standard error an
// ExitError keeps.
const maxStderr = 8 << 10

// Runner is a runner of commands in the VM uuid.
func (d *Dialer) Runner(vm string) Runner {
	return Runner{dialer: d, vm: vm}
}

// Run runs command to its end: stdin is its standard input, which nil
// leaves empty, and its standard output is written to stdout. A command
// that exits with a status other than 0 is an *ExitError saying what it
// said on its standard error; one stopped by ctx is ctx's error.
func (r Runner) Run(ctx context.Context, command []string, stdin io.Reader, stdout io.Writer) error {
	stderr := &firstBytes{limit: maxStderr}

	session, err := r.dialer.Exec(ctx, r.vm, ExecOptions{Command: command, Stderr: stderr})
	if err != nil {
		return err
	}

	defer func() { _ = session.Close() }()

	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()

	unread := make(chan error, 1)
	go feed(session, stdin, unread)

	if _, err := io.Copy(stdout, session); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		select {
		case inputErr := <-unread:
			return inputErr
		default:
		}

		return err
	}

	code, err := session.Wait()
	switch {
	case err != nil:
		return err
	case code != 0:
		return &ExitError{Code: code, Stderr: stderr.String()}
	}

	return nil
}

// feed writes in to the command's standard input, then ends it. Input that
// cannot be read stops the command: why is told on unread before the
// command is let go of, so that whoever sees it stop knows why.
func feed(session *Exec, in io.Reader, unread chan<- error) {
	if in != nil {
		buffer := make([]byte, maxMessage)

		for {
			n, err := in.Read(buffer)
			if n > 0 {
				if _, writeErr := session.Write(buffer[:n]); writeErr != nil {
					return
				}
			}

			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				unread <- fmt.Errorf("reading the command's input: %w", err)
				_ = session.Close()

				return
			}
		}
	}

	_ = session.CloseWrite()
}

// firstBytes keeps the first limit bytes written to it.
type firstBytes struct {
	lock  sync.Mutex
	limit int
	data  []byte
}

// Write keeps what of p fits.
func (f *firstBytes) Write(p []byte) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if room := f.limit - len(f.data); room > 0 {
		f.data = append(f.data, p[:min(room, len(p))]...)
	}

	return len(p), nil
}

// String is what was kept.
func (f *firstBytes) String() string {
	f.lock.Lock()
	defer f.lock.Unlock()

	return string(f.data)
}

// Relay opens a TCP connection to port of host as the VM uuid sees them,
// relayed by nc there: a port the VM does not publish, or one of a
// container in it. Its writing half is closed by CloseWrite, and it reads
// an error when nc fails.
func (d *Dialer) Relay(ctx context.Context, vm, host string, port uint) (*Relayed, error) {
	stderr := &firstBytes{limit: maxStderr}

	session, err := d.Exec(ctx, vm, ExecOptions{Command: []string{"nc", host, fmt.Sprint(port)}, Stderr: stderr})
	if err != nil {
		return nil, err
	}

	return &Relayed{session: session, stderr: stderr}, nil
}

// Relayed is a TCP connection relayed by nc in a VM.
type Relayed struct {
	session *Exec
	stderr  *firstBytes
}

// Read reads what the other end sent, and is io.EOF once it closed the
// connection, or an *ExitError when nc failed.
func (r *Relayed) Read(p []byte) (int, error) {
	n, err := r.session.Read(p)
	if !errors.Is(err, io.EOF) {
		return n, err
	}

	code, waitErr := r.session.Wait()
	switch {
	case waitErr != nil:
		return n, waitErr
	case code != 0:
		return n, &ExitError{Code: code, Stderr: r.stderr.String()}
	}

	return n, io.EOF
}

// Write sends p to the other end.
func (r *Relayed) Write(p []byte) (int, error) {
	return r.session.Write(p)
}

// CloseWrite closes the connection's writing half: the other end reads its
// end, and may still answer.
func (r *Relayed) CloseWrite() error {
	return r.session.CloseWrite()
}

// Close closes the connection.
func (r *Relayed) Close() error {
	return r.session.Close()
}
