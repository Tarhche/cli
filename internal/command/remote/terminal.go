package remote

import (
	"context"
	"errors"
	"io"

	"golang.org/x/term"

	"github.com/Tarhche/cli/internal/command"
)

// errNotTerminal is a stream asked about as a terminal that is not one.
var errNotTerminal = errors.New("not a terminal")

// terminal is the local terminal a remote one is shown on.
type terminal interface {
	// IsTerminal reports whether stream is a terminal.
	IsTerminal(stream any) bool

	// MakeRaw puts the local terminal in raw mode, every key read from in
	// as it is typed and out taking a remote terminal's escape sequences,
	// and is what puts it back.
	MakeRaw(in, out any) (func() error, error)

	// Size is how many rows and columns the terminal stream is.
	Size(stream any) (rows, cols uint, err error)

	// Resized tells, until ctx is done, that the terminal may have been
	// resized; the channel is closed once ctx is done.
	Resized(ctx context.Context) <-chan struct{}
}

// localTerminal is the terminal the CLI runs in.
type localTerminal struct{}

var _ terminal = localTerminal{}

func (localTerminal) IsTerminal(stream any) bool {
	return command.IsTerminal(stream)
}

func (localTerminal) MakeRaw(in, out any) (func() error, error) {
	fd, ok := descriptor(in)
	if !ok {
		return nil, errNotTerminal
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	restoreOutput := virtualTerminal(out)

	return func() error { return errors.Join(restoreOutput(), term.Restore(fd, state)) }, nil
}

func (localTerminal) Size(stream any) (uint, uint, error) {
	fd, ok := descriptor(stream)
	if !ok {
		return 0, 0, errNotTerminal
	}

	cols, rows, err := term.GetSize(fd)
	if err != nil {
		return 0, 0, err
	}

	return uint(rows), uint(cols), nil
}

func (localTerminal) Resized(ctx context.Context) <-chan struct{} {
	return resized(ctx)
}

// descriptor is the file descriptor of stream, when it is a file.
func descriptor(stream any) (int, bool) {
	file, ok := stream.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}

	return int(file.Fd()), true
}

// session is what the local terminal is attached to: a remote terminal, or
// a command, on a terminal or not.
type session interface {
	io.ReadWriteCloser
	Resize(rows, cols uint) error
}

// attachment is how a session is attached to the local terminal.
type attachment struct {
	// tty says the session is a terminal: the local one is put in raw mode
	// for it when the local input is one, and it is sized as the local
	// output is.
	tty bool

	// rows and cols are the size the session was opened with.
	rows, cols uint

	// input says the local input is written to the session.
	input bool

	// end is told once the local input ended.
	end func() error
}

// attach runs a session until its output ends: the local input written to
// it, and its output to the local output. A session stopped by ctx is
// ctx's error.
func (r *remote) attach(ctx context.Context, s session, a attachment) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := context.AfterFunc(ctx, func() { _ = s.Close() })
	defer stop()

	if a.tty && a.input && r.terminal.IsTerminal(r.cli.In) {
		restore, err := r.terminal.MakeRaw(r.cli.In, r.cli.Out)
		if err != nil {
			return err
		}

		defer func() { _ = restore() }()
	}

	if a.tty && r.terminal.IsTerminal(r.cli.Out) {
		r.followSize(ctx, s, a.rows, a.cols)
	}

	if a.input {
		go func() {
			if _, err := io.Copy(s, r.cli.In); err == nil && a.end != nil {
				_ = a.end()
			}
		}()
	}

	if _, err := io.Copy(r.cli.Out, s); err != nil && ctx.Err() == nil {
		return err
	}

	return ctx.Err()
}

// followSize tells s the local terminal's size now and whenever it changes,
// until ctx is done, rows and cols being what it was told last.
func (r *remote) followSize(ctx context.Context, s session, rows, cols uint) {
	tell := func() {
		newRows, newCols, err := r.terminal.Size(r.cli.Out)
		if err != nil || newRows == rows && newCols == cols {
			return
		}

		if s.Resize(newRows, newCols) == nil {
			rows, cols = newRows, newCols
		}
	}

	tell()

	resized := r.terminal.Resized(ctx)

	go func() {
		for range resized {
			tell()
		}
	}()
}

// size is the local output's size, nothing when it is not a terminal.
func (r *remote) size() (uint, uint) {
	if !r.terminal.IsTerminal(r.cli.Out) {
		return 0, 0
	}

	rows, cols, err := r.terminal.Size(r.cli.Out)
	if err != nil {
		return 0, 0
	}

	return rows, cols
}
