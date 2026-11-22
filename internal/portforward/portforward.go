// Package portforward forwards local TCP ports to a workload's: a forward
// listens on a local address, and pipes every connection it accepts to one
// its dial opens on the other side, whatever carries it there. A Forwarder
// keeps the forwards it started, so that they can be listed and stopped.
package portforward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

// DefaultAddress is the address a forward listens on unless it names one.
const DefaultAddress = "127.0.0.1"

// copyBuffer is how much is copied at a time, each way.
const copyBuffer = 32 << 10

// Dial opens the other side of a connection a forward accepted. It is
// called once for every connection, and its context ends when the forward
// is stopped.
type Dial func(ctx context.Context) (io.ReadWriteCloser, error)

// Spec is a forward to start.
type Spec struct {
	// Address is the local address to listen on, DefaultAddress when it is
	// empty.
	Address string

	// Port is the local port to listen on, any free one when it is 0.
	Port uint16

	// Remote names the other side, as the forward is listed: "8080", say.
	Remote string

	// Dial opens the other side of each connection.
	Dial Dial
}

// EventKind is what happened to a connection.
type EventKind int

const (
	// Accepted is a connection accepted, about to be dialled.
	Accepted EventKind = iota + 1

	// Failed is a connection whose other side could not be dialled, or
	// stopped short.
	Failed

	// Closed is a connection that ended, failed or not.
	Closed
)

// String names the kind.
func (k EventKind) String() string {
	switch k {
	case Accepted:
		return "accepted"
	case Failed:
		return "failed"
	case Closed:
		return "closed"
	}

	return "EventKind(" + strconv.Itoa(int(k)) + ")"
}

// Event is something that happened to a connection of a forward.
type Event struct {
	Kind    EventKind
	Forward *Forward

	// Client is the local peer of the connection.
	Client net.Addr

	// Err is why it failed.
	Err error
}

// Forwarder starts forwards and keeps the ones that run.
type Forwarder struct {
	lock     sync.Mutex
	last     int
	forwards []*Forward

	events func(Event)
	listen func(ctx context.Context, network, address string) (net.Listener, error)
}

// Option configures a Forwarder.
type Option func(*Forwarder)

// WithEvents tells handle what happens to every connection of every forward.
// It is called from the goroutines connections are served on, so it has to
// be safe to call from several at once, and must not wait for a forward to
// close, which waits for those goroutines.
func WithEvents(handle func(Event)) Option {
	return func(f *Forwarder) { f.events = handle }
}

// WithListener listens through listen instead of a net.ListenConfig.
func WithListener(listen func(ctx context.Context, network, address string) (net.Listener, error)) Option {
	return func(f *Forwarder) { f.listen = listen }
}

// New is a forwarder that runs no forward yet.
func New(options ...Option) *Forwarder {
	f := &Forwarder{
		events: func(Event) {},
		listen: (&net.ListenConfig{}).Listen,
	}

	for _, option := range options {
		option(f)
	}

	return f
}

// Start starts forwarding spec, and is the forward started, listening once
// Start returns.
func (f *Forwarder) Start(spec Spec) (*Forward, error) {
	if spec.Dial == nil {
		return nil, errors.New("portforward: a forward needs a dial")
	}

	if len(spec.Address) == 0 {
		spec.Address = DefaultAddress
	}

	address := net.JoinHostPort(spec.Address, strconv.Itoa(int(spec.Port)))

	listener, err := f.listen(context.Background(), "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", address, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &Forward{
		spec:      spec,
		forwarder: f,
		listener:  listener,
		ctx:       ctx,
		cancel:    cancel,
		conns:     map[net.Conn]struct{}{},
		done:      make(chan struct{}),
	}

	f.lock.Lock()
	f.last++
	fw.ID = f.last
	f.forwards = append(f.forwards, fw)
	f.lock.Unlock()

	go fw.serve()

	return fw, nil
}

// Forwards are the forwards running, in the order they were started.
func (f *Forwarder) Forwards() []*Forward {
	f.lock.Lock()
	defer f.lock.Unlock()

	return append([]*Forward(nil), f.forwards...)
}

// Forward is the running forward of that id, and whether there is one.
func (f *Forwarder) Forward(id int) (*Forward, bool) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, fw := range f.forwards {
		if fw.ID == id {
			return fw, true
		}
	}

	return nil, false
}

// Close stops every forward, and returns once they stopped.
func (f *Forwarder) Close() error {
	var errs []error

	for _, fw := range f.Forwards() {
		errs = append(errs, fw.Close())
	}

	return errors.Join(errs...)
}

func (f *Forwarder) forget(fw *Forward) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for i, running := range f.forwards {
		if running == fw {
			f.forwards = append(f.forwards[:i], f.forwards[i+1:]...)

			return
		}
	}
}

// Forward is a local address forwarded to the other side.
type Forward struct {
	// ID is what its forwarder knows it by.
	ID int

	spec      Spec
	forwarder *Forwarder
	listener  net.Listener

	ctx    context.Context
	cancel context.CancelFunc

	lock        sync.Mutex
	conns       map[net.Conn]struct{}
	connections sync.WaitGroup
	stopping    atomic.Bool
	err         error

	closeOnce sync.Once
	closeErr  error
	done      chan struct{}
}

// Addr is the local address it listens on.
func (fw *Forward) Addr() net.Addr {
	return fw.listener.Addr()
}

// Remote names the other side.
func (fw *Forward) Remote() string {
	return fw.spec.Remote
}

// Connections is how many connections it is serving now.
func (fw *Forward) Connections() int {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	return len(fw.conns)
}

// Done is closed once it stopped, its connections ended: it was closed, or
// it could not accept connections any more.
func (fw *Forward) Done() <-chan struct{} {
	return fw.done
}

// Err is why it stopped accepting connections on its own, once Done is
// closed; nil when it was closed.
func (fw *Forward) Err() error {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	return fw.err
}

// Close stops listening, ends its connections, and returns once they ended.
func (fw *Forward) Close() error {
	fw.closeOnce.Do(func() {
		fw.stopping.Store(true)
		fw.cancel()

		if err := fw.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fw.closeErr = err
		}

		fw.closeConnections()
	})

	<-fw.done

	return fw.closeErr
}

// serve accepts connections until the listener stops, and serves each on a
// goroutine of its own.
func (fw *Forward) serve() {
	defer close(fw.done)
	defer fw.forwarder.forget(fw)
	defer fw.connections.Wait()

	for {
		conn, err := fw.listener.Accept()
		if err != nil {
			if !fw.stopping.Load() {
				fw.lock.Lock()
				fw.err = err
				fw.lock.Unlock()

				fw.cancel()
				fw.closeConnections()
			}

			return
		}

		fw.track(conn)

		go fw.handle(conn)
	}
}

func (fw *Forward) track(conn net.Conn) {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	fw.conns[conn] = struct{}{}
	fw.connections.Add(1)
}

func (fw *Forward) untrack(conn net.Conn) {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	delete(fw.conns, conn)
	fw.connections.Done()
}

func (fw *Forward) closeConnections() {
	fw.lock.Lock()
	defer fw.lock.Unlock()

	for conn := range fw.conns {
		_ = conn.Close()
	}
}

// handle pipes local to the other side its dial opens, and tells what
// happens to it.
func (fw *Forward) handle(local net.Conn) {
	defer fw.untrack(local)
	defer func() { _ = local.Close() }()

	client := local.RemoteAddr()
	fw.emit(Accepted, client, nil)

	remote, err := fw.spec.Dial(fw.ctx)
	if err != nil {
		if !fw.stopping.Load() {
			fw.emit(Failed, client, err)
		}

		fw.emit(Closed, client, nil)

		return
	}

	if err := pipe(local, remote); err != nil && !fw.stopping.Load() {
		fw.emit(Failed, client, err)
	}

	fw.emit(Closed, client, nil)
}

func (fw *Forward) emit(kind EventKind, client net.Addr, err error) {
	fw.forwarder.events(Event{Kind: kind, Forward: fw, Client: client, Err: err})
}

// halfCloser is a connection whose writing half closes on its own.
type halfCloser interface {
	CloseWrite() error
}

// pipe copies bytes both ways between local and remote until both are done,
// and is what stopped the other side short, if anything did: what the local
// side does is its own business.
//
// The local side's end closes the remote side's writing half when it has
// one, so that its answer is still read, and closes it otherwise. The
// remote side's end is the connection's.
func pipe(local net.Conn, remote io.ReadWriteCloser) error {
	var (
		closing   atomic.Bool
		failure   error
		failOnce  sync.Once
		waitGroup sync.WaitGroup
	)

	fail := func(err error) {
		if err != nil && !closing.Load() && !errors.Is(err, net.ErrClosed) {
			failOnce.Do(func() { failure = err })
		}
	}

	closeBoth := func() {
		closing.Store(true)

		_ = local.Close()
		_ = remote.Close()
	}

	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()

		readErr, writeErr := copyBytes(remote, local)
		if writeErr != nil {
			fail(writeErr)
		}

		if readErr != nil || writeErr != nil {
			closeBoth()

			return
		}

		if half, ok := remote.(halfCloser); ok {
			if err := half.CloseWrite(); err != nil {
				fail(err)
				closeBoth()
			}

			return
		}

		closeBoth()
	}()

	go func() {
		defer waitGroup.Done()

		readErr, _ := copyBytes(local, remote)
		fail(readErr)
		closeBoth()
	}()

	waitGroup.Wait()

	return failure
}

// copyBytes copies from src to dst until src ends, and is the error reading
// src or the error writing dst that stopped it, if either did.
func copyBytes(dst io.Writer, src io.Reader) (readErr, writeErr error) {
	buffer := make([]byte, copyBuffer)

	for {
		n, err := src.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return nil, writeErr
			}
		}

		if errors.Is(err, io.EOF) {
			return nil, nil
		}

		if err != nil {
			return err, nil
		}
	}
}
