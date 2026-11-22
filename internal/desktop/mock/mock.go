// Package mock holds test doubles of what the desktop app is given: how it
// signs in, reaches into workloads, asks for paths on the host and tells
// its frontend what happened.
package mock

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	testify "github.com/stretchr/testify/mock"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/desktop"
)

// patience bounds how long a test waits for something to happen.
const patience = 5 * time.Second

var (
	_ desktop.Authenticator  = (*Authenticator)(nil)
	_ desktop.TerminalOpener = (*TerminalOpener)(nil)
	_ desktop.Terminal       = (*Terminal)(nil)
	_ desktop.Forwarder      = (*Forwarder)(nil)
	_ desktop.PortForward    = (*PortForward)(nil)
	_ desktop.Copier         = (*Copier)(nil)
	_ desktop.Chooser        = (*Chooser)(nil)
	_ desktop.Emitter        = (*Emitter)(nil)
)

// Authenticator signs in and out as a test expects.
type Authenticator struct {
	testify.Mock
}

// LoginWithBrowser is called as a test expects.
func (m *Authenticator) LoginWithBrowser(ctx context.Context, contextName string) error {
	return m.Called(ctx, contextName).Error(0)
}

// LoginWithPassword is called as a test expects.
func (m *Authenticator) LoginWithPassword(ctx context.Context, contextName, identity, password string) error {
	return m.Called(ctx, contextName, identity, password).Error(0)
}

// Logout is called as a test expects.
func (m *Authenticator) Logout(ctx context.Context, contextName string) error {
	return m.Called(ctx, contextName).Error(0)
}

// Status is called as a test expects.
func (m *Authenticator) Status(ctx context.Context, contextName string) (auth.Status, error) {
	args := m.Called(ctx, contextName)

	status, _ := args.Get(0).(auth.Status)

	return status, args.Error(1)
}

// TerminalOpener opens terminals as a test expects.
type TerminalOpener struct {
	testify.Mock
}

// Open is called as a test expects.
func (m *TerminalOpener) Open(ctx context.Context, cli *command.Cli, request desktop.TerminalRequest) (desktop.Terminal, error) {
	args := m.Called(ctx, cli, request)

	terminal, _ := args.Get(0).(desktop.Terminal)

	return terminal, args.Error(1)
}

// Forwarder forwards ports as a test expects.
type Forwarder struct {
	testify.Mock
}

// Forward is called as a test expects.
func (m *Forwarder) Forward(ctx context.Context, cli *command.Cli, request desktop.ForwardRequest) (desktop.PortForward, error) {
	args := m.Called(ctx, cli, request)

	forward, _ := args.Get(0).(desktop.PortForward)

	return forward, args.Error(1)
}

// Copier copies as a test expects.
type Copier struct {
	testify.Mock
}

// CopyIn is called as a test expects.
func (m *Copier) CopyIn(ctx context.Context, cli *command.Cli, request desktop.CopyRequest) error {
	return m.Called(ctx, cli, request).Error(0)
}

// CopyOut is called as a test expects.
func (m *Copier) CopyOut(ctx context.Context, cli *command.Cli, request desktop.CopyRequest) error {
	return m.Called(ctx, cli, request).Error(0)
}

// Chooser chooses paths as a test expects.
type Chooser struct {
	testify.Mock
}

// Open is called as a test expects.
func (m *Chooser) Open(title string) (string, error) {
	args := m.Called(title)

	return args.String(0), args.Error(1)
}

// Save is called as a test expects.
func (m *Chooser) Save(title, filename string) (string, error) {
	args := m.Called(title, filename)

	return args.String(0), args.Error(1)
}

// Terminal is a terminal a test says what it says, and hears what is typed
// into it.
type Terminal struct {
	// WriteErr, ResizeErr and CloseErr are what writing, resizing and
	// closing it fail with.
	WriteErr  error
	ResizeErr error
	CloseErr  error

	said      chan reading
	closed    chan struct{}
	closeOnce sync.Once

	lock  sync.Mutex
	input bytes.Buffer
	sizes []desktop.TerminalSize
}

// reading is what one read of a terminal answers.
type reading struct {
	data []byte
	err  error
}

// NewTerminal is a terminal that says nothing until a test says it.
func NewTerminal() *Terminal {
	return &Terminal{said: make(chan reading, 16), closed: make(chan struct{})}
}

// Say makes the terminal say data.
func (t *Terminal) Say(data []byte) {
	t.said <- reading{data: data}
}

// End makes the terminal end, reading it then failing with err: io.EOF for
// a terminal that ended well.
func (t *Terminal) End(err error) {
	t.said <- reading{err: err}
}

// Read answers what the terminal was made to say, in order, and fails once
// it is closed.
func (t *Terminal) Read(p []byte) (int, error) {
	select {
	case r := <-t.said:
		return copy(p, r.data), r.err
	case <-t.closed:
		return 0, io.ErrClosedPipe
	}
}

// Write hears what is typed.
func (t *Terminal) Write(p []byte) (int, error) {
	if t.WriteErr != nil {
		return 0, t.WriteErr
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	return t.input.Write(p)
}

// Resize hears the sizes it is given.
func (t *Terminal) Resize(size desktop.TerminalSize) error {
	if t.ResizeErr != nil {
		return t.ResizeErr
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	t.sizes = append(t.sizes, size)

	return nil
}

// Close closes it, which reading it then fails with.
func (t *Terminal) Close() error {
	t.closeOnce.Do(func() { close(t.closed) })

	return t.CloseErr
}

// Input is what was typed into it.
func (t *Terminal) Input() string {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.input.String()
}

// Sizes are the sizes it was given, in order.
func (t *Terminal) Sizes() []desktop.TerminalSize {
	t.lock.Lock()
	defer t.lock.Unlock()

	return append([]desktop.TerminalSize(nil), t.sizes...)
}

// IsClosed reports whether it was closed.
func (t *Terminal) IsClosed() bool {
	select {
	case <-t.closed:
		return true
	default:
		return false
	}
}

// PortForward is a forward a test stops, or makes stop by itself.
type PortForward struct {
	// CloseErr is what closing it fails with.
	CloseErr error

	address  string
	stopped  chan struct{}
	stopOnce sync.Once
	err      error
}

// NewPortForward is a forward listening at address until it is stopped.
func NewPortForward(address string) *PortForward {
	return &PortForward{address: address, stopped: make(chan struct{})}
}

// Address is where it listens.
func (f *PortForward) Address() string {
	return f.address
}

// Wait waits until it stops, and is what it stopped by itself with.
func (f *PortForward) Wait() error {
	<-f.stopped

	return f.err
}

// Close stops it.
func (f *PortForward) Close() error {
	f.stop(nil)

	return f.CloseErr
}

// Fail makes it stop by itself, saying err.
func (f *PortForward) Fail(err error) {
	f.stop(err)
}

// IsStopped reports whether it stopped.
func (f *PortForward) IsStopped() bool {
	select {
	case <-f.stopped:
		return true
	default:
		return false
	}
}

func (f *PortForward) stop(err error) {
	f.stopOnce.Do(func() {
		f.err = err
		close(f.stopped)
	})
}

// Event is something an Emitter was told of.
type Event struct {
	Name string
	Data any
}

// Emitter hears the events the desktop tells its frontend of, in order.
type Emitter struct {
	events chan Event
}

// NewEmitter is an emitter that heard nothing yet.
func NewEmitter() *Emitter {
	return &Emitter{events: make(chan Event, 256)}
}

// Emit hears an event, carrying the one data it is given, or all of it.
func (e *Emitter) Emit(name string, data ...any) bool {
	var carried any = data
	if len(data) == 1 {
		carried = data[0]
	}

	e.events <- Event{Name: name, Data: carried}

	return false
}

// Next is the next event it heard, which t waits a while for.
func (e *Emitter) Next(t testing.TB) Event {
	t.Helper()

	select {
	case event := <-e.events:
		return event
	case <-time.After(patience):
		t.Fatal("no event was emitted")

		return Event{}
	}
}

// Quiet reports whether it heard nothing more for a little while.
func (e *Emitter) Quiet() bool {
	select {
	case <-e.events:
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}
