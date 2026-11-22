package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/auth"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
)

// removeContextsCommand is docker's context rm: contexts forgotten, and
// logged out of, but for the one the CLI talks to unless it is forced.
type removeContextsCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	force bool
}

func newRemoveContextsCommand(cli *command.Cli) *removeContextsCommand {
	return &removeContextsCommand{cli: cli}
}

func (c *removeContextsCommand) Name() string {
	return "rm"
}

func (c *removeContextsCommand) Description() string {
	return "Remove one or more contexts"
}

func (c *removeContextsCommand) Usage() string {
	return command.Name + " context rm [OPTIONS] CONTEXT [CONTEXT...]"
}

func (c *removeContextsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.force, console.Long("force"), "Force the removal of a context in use", console.Short("f"))
}

func (c *removeContextsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeContextsCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	current, err := c.cli.ContextName()
	if err != nil {
		return err
	}

	manager, err := c.cli.Auth()
	if err != nil {
		return err
	}

	var (
		failures []error
		removed  []string
	)

	for _, name := range c.flags.Args() {
		if err := c.remove(ctx, cfg, manager, name, name == current); err != nil {
			failures = append(failures, err)

			continue
		}

		removed = append(removed, name)
	}

	if len(removed) > 0 {
		if err := cfg.Save(); err != nil {
			return err
		}
	}

	for _, name := range removed {
		if _, err := fmt.Fprintln(c.cli.Out, name); err != nil {
			return err
		}
	}

	return errors.Join(failures...)
}

// remove forgets a context, logging out of it first. Forcing it removes the
// context in use, and is no error for one that is not there, whose session,
// if any is left, is forgotten all the same.
func (c *removeContextsCommand) remove(ctx context.Context, cfg *config.File, manager *auth.Manager, name string, inUse bool) error {
	if name == config.DefaultContextName {
		return fmt.Errorf("context %q cannot be removed", name)
	}

	if inUse && !c.force {
		return fmt.Errorf("context %q is in use, set -f flag to force remove", name)
	}

	_, exists := cfg.Contexts[name]
	if !exists && !c.force {
		return fmt.Errorf("%w: %q", config.ErrNoSuchContext, name)
	}

	if err := manager.Logout(ctx, name); err != nil {
		return err
	}

	if !exists {
		return nil
	}

	return cfg.RemoveContext(name)
}
