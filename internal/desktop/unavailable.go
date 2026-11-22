package desktop

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tarhche/cli/internal/command"
)

// ErrUnavailable is what reaching into a workload says until the desktop is
// given a way to.
var ErrUnavailable = errors.New("not available yet")

// Unavailable reaches into no workload: it is the desktop's terminals, port
// forwards and copies until they are built in, and refuses each with
// ErrUnavailable.
type Unavailable struct{}

var (
	_ TerminalOpener = Unavailable{}
	_ Forwarder      = Unavailable{}
	_ Copier         = Unavailable{}
)

// Open refuses to open a terminal.
func (Unavailable) Open(context.Context, *command.Cli, TerminalRequest) (Terminal, error) {
	return nil, fmt.Errorf("terminals are %w", ErrUnavailable)
}

// Forward refuses to forward a port.
func (Unavailable) Forward(context.Context, *command.Cli, ForwardRequest) (PortForward, error) {
	return nil, fmt.Errorf("port forwards are %w", ErrUnavailable)
}

// CopyIn refuses to copy into a workload.
func (Unavailable) CopyIn(context.Context, *command.Cli, CopyRequest) error {
	return fmt.Errorf("copying files is %w", ErrUnavailable)
}

// CopyOut refuses to copy out of a workload.
func (Unavailable) CopyOut(context.Context, *command.Cli, CopyRequest) error {
	return fmt.Errorf("copying files is %w", ErrUnavailable)
}
