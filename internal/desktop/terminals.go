package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/Tarhche/cli/internal/command"
)

// terminalChunk bounds how much of what a terminal says one event carries.
const terminalChunk = 32 << 10

// Target is what a terminal, a forwarded port or a copy reaches into: a VM,
// or a container in a Docker VM.
type Target struct {
	// VM is the VM's uuid: for a container, the Docker VM it lives in.
	VM string `json:"vm"`

	// Container names a container in the VM, by its uuid, its Docker id or
	// its name; empty is the VM itself.
	Container string `json:"container,omitempty"`
}

// TerminalSize is how many rows and columns a terminal has.
type TerminalSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// TerminalRequest is a terminal to open.
type TerminalRequest struct {
	Target Target `json:"target"`

	// Command is what runs on it; empty is a shell: a VM's own (its attach
	// stream), or one run in the container.
	Command []string `json:"command,omitempty"`

	Size TerminalSize `json:"size"`
}

// Terminal is a terminal open in a workload.
type Terminal interface {
	// Read reads what the terminal says: io.EOF once it has ended, and a
	// command.StatusError when what ran on it exited with another status
	// than zero.
	Read(p []byte) (int, error)

	// Write types p into the terminal.
	Write(p []byte) (int, error)

	// Resize tells the terminal its new size.
	Resize(size TerminalSize) error

	// Close ends the terminal.
	Close() error
}

// TerminalOpener opens terminals in workloads: a VM's shell (attach), or a
// command on a terminal in a VM or a container (exec), through the
// workload ingress of the context cli talks to.
type TerminalOpener interface {
	// Open opens a terminal that lives as long as ctx, or until it is
	// closed.
	Open(ctx context.Context, cli *command.Cli, request TerminalRequest) (Terminal, error)
}

// Terminals is how the frontend reaches terminals in workloads: it opens
// them, types into them and resizes them, and is told what they say in
// EventTerminalOutput events, and that they ended in an EventTerminalExit.
type Terminals struct {
	ctx       context.Context
	workspace *Workspace
	opener    TerminalOpener
	emitter   Emitter

	lock     sync.Mutex
	sessions map[string]*terminalSession
}

// terminalSession is an open terminal, and whether the frontend closed it.
type terminalSession struct {
	terminal Terminal
	cancel   context.CancelFunc
	closed   atomic.Bool
}

// NewTerminals is the terminal service of workspace: every terminal lives
// at most as long as ctx, is opened by opener, and is heard through
// emitter.
func NewTerminals(ctx context.Context, workspace *Workspace, opener TerminalOpener, emitter Emitter) *Terminals {
	return &Terminals{
		ctx:       ctx,
		workspace: workspace,
		opener:    opener,
		emitter:   emitter,
		sessions:  make(map[string]*terminalSession),
	}
}

// Open opens a terminal under id, which the frontend chose and listens to
// its events by; what it says is told from then on.
func (t *Terminals) Open(id string, request TerminalRequest) error {
	if len(id) == 0 {
		return InvalidError{Fields: map[string]string{"id": "a terminal needs an id"}}
	}

	ctx, cancel := context.WithCancel(t.ctx)

	terminal, err := t.opener.Open(ctx, t.workspace.Cli(), request)
	if err != nil {
		cancel()

		return err
	}

	session := &terminalSession{terminal: terminal, cancel: cancel}

	t.lock.Lock()
	_, taken := t.sessions[id]
	if !taken {
		t.sessions[id] = session
	}
	t.lock.Unlock()

	if taken {
		cancel()
		_ = terminal.Close()

		return fmt.Errorf("a terminal %q is open already", id)
	}

	go t.relay(id, session)

	return nil
}

// Write types data into the terminal id.
func (t *Terminals) Write(id string, data []byte) error {
	session, err := t.session(id)
	if err != nil {
		return err
	}

	_, err = session.terminal.Write(data)

	return err
}

// Resize tells the terminal id its new size.
func (t *Terminals) Resize(id string, size TerminalSize) error {
	session, err := t.session(id)
	if err != nil {
		return err
	}

	return session.terminal.Resize(size)
}

// Close ends the terminal id; closing one that ended is no error.
func (t *Terminals) Close(id string) error {
	session, found := t.lookup(id)
	if !found {
		return nil
	}

	session.closed.Store(true)
	session.cancel()

	return session.terminal.Close()
}

// ServiceShutdown ends every terminal as the app quits.
func (t *Terminals) ServiceShutdown() error {
	t.lock.Lock()
	sessions := make([]*terminalSession, 0, len(t.sessions))
	for _, session := range t.sessions {
		sessions = append(sessions, session)
	}
	t.lock.Unlock()

	var closing []error
	for _, session := range sessions {
		session.closed.Store(true)
		session.cancel()
		closing = append(closing, session.terminal.Close())
	}

	return errors.Join(closing...)
}

func (t *Terminals) session(id string) (*terminalSession, error) {
	session, found := t.lookup(id)
	if !found {
		return nil, fmt.Errorf("there is no terminal %q", id)
	}

	return session, nil
}

func (t *Terminals) lookup(id string) (*terminalSession, bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	session, found := t.sessions[id]

	return session, found
}

// relay tells the frontend what the terminal id says until it ends, and
// then that it ended.
func (t *Terminals) relay(id string, session *terminalSession) {
	chunk := make([]byte, terminalChunk)

	for {
		n, err := session.terminal.Read(chunk)
		if n > 0 {
			t.emitter.Emit(EventTerminalOutput, TerminalOutput{ID: id, Data: bytes.Clone(chunk[:n])})
		}

		if err != nil {
			t.end(id, session, err)

			return
		}
	}
}

func (t *Terminals) end(id string, session *terminalSession, err error) {
	t.lock.Lock()
	if t.sessions[id] == session {
		delete(t.sessions, id)
	}
	t.lock.Unlock()

	session.cancel()
	_ = session.terminal.Close()

	t.emitter.Emit(EventTerminalExit, exitOf(id, err, session.closed.Load()))
}

// exitOf is how a terminal ended, err being what reading it last said: well
// once it was closed or said all it had, with the status of what ran on it,
// or failing.
func exitOf(id string, err error, closed bool) TerminalExit {
	var status command.StatusError

	switch {
	case closed, errors.Is(err, io.EOF):
		return TerminalExit{ID: id}
	case errors.As(err, &status):
		return TerminalExit{ID: id, Code: status.Code, Error: status.Message}
	default:
		return TerminalExit{ID: id, Code: -1, Error: err.Error()}
	}
}
