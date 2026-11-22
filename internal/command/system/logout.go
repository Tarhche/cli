package system

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// logoutCommand is docker's logout: it forgets the session of the context
// the CLI talks to, and ends it on its estate.
type logoutCommand struct {
	cli   *command.Cli
	flags *console.FlagSet
}

func newLogoutCommand(cli *command.Cli) *logoutCommand {
	return &logoutCommand{cli: cli}
}

func (c *logoutCommand) Name() string {
	return "logout"
}

func (c *logoutCommand) Description() string {
	return "Log out from the estate of the current context"
}

func (c *logoutCommand) Usage() string {
	return command.Name + " logout"
}

func (c *logoutCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
}

func (c *logoutCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *logoutCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	current, err := c.cli.Context()
	if err != nil {
		return err
	}

	manager, err := c.cli.Auth()
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(c.cli.Out, "Removing login credentials for %s\n", current.Name); err != nil {
		return err
	}

	return manager.Logout(ctx, current.Name)
}
