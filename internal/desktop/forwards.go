package desktop

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tarhche/cli/internal/command"
)

// ForwardRequest is a port of a workload to forward to the host.
type ForwardRequest struct {
	Target Target `json:"target"`

	// Port is the workload's port: one a VM publishes, or a container's.
	Port uint16 `json:"port"`

	// LocalPort is the port on the host it is forwarded to; zero takes any
	// free one.
	LocalPort uint16 `json:"local_port"`
}

// PortForward is a port being forwarded.
type PortForward interface {
	// Address is where it listens on the host, as host:port.
	Address() string

	// Wait waits until it stops, and says why when it was not closed.
	Wait() error

	// Close stops it.
	Close() error
}

// Forwarder forwards workloads' ports to the host.
type Forwarder interface {
	// Forward listens on the host and carries every connection made there
	// to the workload's port, through the workload ingress of the context
	// cli talks to, until it is closed or ctx is done. It answers once it
	// listens.
	Forward(ctx context.Context, cli *command.Cli, request ForwardRequest) (PortForward, error)
}

// Forward is a port being forwarded, as the frontend lists it.
type Forward struct {
	ID string `json:"id"`

	// Context is the context it was forwarded through.
	Context string `json:"context"`
	Target  Target `json:"target"`
	Port    uint16 `json:"port"`

	// Address is where it listens on the host.
	Address   string    `json:"address"`
	StartedAt time.Time `json:"started_at"`
}

// Forwards is how the frontend forwards ports of workloads to the host: it
// starts, lists and stops them, and is told of one that stopped by itself
// in an EventForwardStopped.
type Forwards struct {
	ctx       context.Context
	workspace *Workspace
	forwarder Forwarder
	emitter   Emitter

	next     atomic.Uint64
	lock     sync.Mutex
	forwards map[string]*forwarding
}

// forwarding is a port being forwarded, and whether the frontend stopped
// it.
type forwarding struct {
	Forward

	forward PortForward
	cancel  context.CancelFunc
	stopped atomic.Bool
}

// NewForwards is the port forward service of workspace: every forward
// lasts at most as long as ctx, is started by forwarder, and is heard of
// through emitter.
func NewForwards(ctx context.Context, workspace *Workspace, forwarder Forwarder, emitter Emitter) *Forwards {
	return &Forwards{
		ctx:       ctx,
		workspace: workspace,
		forwarder: forwarder,
		emitter:   emitter,
		forwards:  make(map[string]*forwarding),
	}
}

// List are the ports being forwarded, the first started first.
func (f *Forwards) List() []Forward {
	f.lock.Lock()
	defer f.lock.Unlock()

	listed := make([]Forward, 0, len(f.forwards))
	for _, forwarding := range f.forwards {
		listed = append(listed, forwarding.Forward)
	}

	slices.SortFunc(listed, func(a, b Forward) int {
		return cmp.Or(a.StartedAt.Compare(b.StartedAt), cmp.Compare(a.ID, b.ID))
	})

	return listed
}

// Start forwards a workload's port to the host, through the context the app
// talks to.
func (f *Forwards) Start(request ForwardRequest) (Forward, error) {
	cli := f.workspace.Cli()

	contextName, err := cli.ContextName()
	if err != nil {
		return Forward{}, err
	}

	ctx, cancel := context.WithCancel(f.ctx)

	forward, err := f.forwarder.Forward(ctx, cli, request)
	if err != nil {
		cancel()

		return Forward{}, err
	}

	started := &forwarding{
		ID:        strconv.FormatUint(f.next.Add(1), 10),
		Context:   contextName,
		Target:    request.Target,
		Port:      request.Port,
		Address:   forward.Address(),
		StartedAt: time.Now(),
		forward:   forward,
		cancel:    cancel,
	}

	f.lock.Lock()
	f.forwards[started.ID] = started
	f.lock.Unlock()

	go f.watch(started)

	return started.Forward, nil
}

// Stop stops the forward id.
func (f *Forwards) Stop(id string) error {
	f.lock.Lock()
	stopping, found := f.forwards[id]
	f.lock.Unlock()

	if !found {
		return fmt.Errorf("there is no port forward %q", id)
	}

	stopping.stopped.Store(true)
	stopping.cancel()

	return stopping.forward.Close()
}

// ServiceShutdown stops every forward as the app quits.
func (f *Forwards) ServiceShutdown() error {
	f.lock.Lock()
	forwards := make([]*forwarding, 0, len(f.forwards))
	for _, forwarding := range f.forwards {
		forwards = append(forwards, forwarding)
	}
	f.lock.Unlock()

	var stopping []error
	for _, forwarding := range forwards {
		forwarding.stopped.Store(true)
		forwarding.cancel()
		stopping = append(stopping, forwarding.forward.Close())
	}

	return errors.Join(stopping...)
}

// watch forgets a forward once it stops, and tells the frontend when it
// stopped by itself.
func (f *Forwards) watch(watched *forwarding) {
	err := watched.forward.Wait()

	f.lock.Lock()
	delete(f.forwards, watched.ID)
	f.lock.Unlock()

	watched.cancel()

	if watched.stopped.Load() {
		return
	}

	stopped := ForwardStopped{ID: watched.ID}
	if err != nil {
		stopped.Error = err.Error()
	}

	f.emitter.Emit(EventForwardStopped, stopped)
}
