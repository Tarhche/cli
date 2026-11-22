package desktop

import (
	"context"
	"sync"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
)

// Opener is the Cli the app talks to a context through.
type Opener func(contextName string) *command.Cli

// NewOpener opens the Cli of a context as the desktop talks to it: sharing
// base's streams, global flags, configuration and sessions, signing in
// through manager, which never asks anything in a terminal. The API and the
// ingress the global flags name are laid over the context the app was
// opened on, and over no other.
func NewOpener(base *command.Cli, cfg *config.File, manager *auth.Manager, openedOn string) Opener {
	return func(contextName string) *command.Cli {
		cli := command.NewCli(base.In, base.Out, base.Err, command.WithConfig(cfg), command.WithAuth(manager))
		cli.Globals = base.Globals
		cli.Globals.Context = contextName

		if contextName != openedOn {
			cli.Globals.Host = ""
			cli.Globals.Ingress = ""
		}

		return cli
	}
}

// Workspace is what the services share: the Cli of the context the app
// talks to, which the user changes, and a lock over the configuration they
// all read and the session service writes.
type Workspace struct {
	lock    sync.RWMutex
	open    Opener
	current *command.Cli
}

// NewWorkspace is a workspace talking to the context of that name first,
// opening the Cli of every context it talks to with open.
func NewWorkspace(contextName string, open Opener) *Workspace {
	return &Workspace{open: open, current: open(contextName)}
}

// Cli is the Cli of the context the app talks to now.
func (w *Workspace) Cli() *command.Cli {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.current
}

// Client is a client of the API of the context the app talks to, signed in.
func (w *Workspace) Client(ctx context.Context) (*api.Client, error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.current.Client(ctx)
}

// ContextName is the name of the context the app talks to.
func (w *Workspace) ContextName() (string, error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.current.ContextName()
}

// Context is the context the app talks to, with what the global flags lay
// over it.
func (w *Workspace) Context() (config.Context, error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.current.Context()
}

// Read calls read with the configuration and the name of the context the
// app talks to, neither of which changes meanwhile.
func (w *Workspace) Read(read func(cfg *config.File, current string) error) error {
	w.lock.RLock()
	defer w.lock.RUnlock()

	cfg, current, err := w.configuration()
	if err != nil {
		return err
	}

	return read(cfg, current)
}

// Change calls change with the configuration and the name of the context
// the app talks to, and saves what it changed. The app talks to the context
// change answers from then on, or to the same one when it answers none.
func (w *Workspace) Change(change func(cfg *config.File, current string) (string, error)) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	cfg, current, err := w.configuration()
	if err != nil {
		return err
	}

	next, err := change(cfg, current)
	if err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	if len(next) > 0 && next != current {
		w.current = w.open(next)
	}

	return nil
}

// Reopen opens the Cli of the context the app talks to afresh, so that what
// signed in or out since is what it talks with.
func (w *Workspace) Reopen() error {
	w.lock.Lock()
	defer w.lock.Unlock()

	current, err := w.current.ContextName()
	if err != nil {
		return err
	}

	w.current = w.open(current)

	return nil
}

func (w *Workspace) configuration() (*config.File, string, error) {
	current, err := w.current.ContextName()
	if err != nil {
		return nil, "", err
	}

	cfg, err := w.current.Config()
	if err != nil {
		return nil, "", err
	}

	return cfg, current, nil
}
