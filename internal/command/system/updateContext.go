package system

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// updateContextCommand is docker's context update: what a context says of
// its estate, changed. A context moved to another API is logged out of the
// estate it was at, and forgets the application the CLI registered there.
type updateContextCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	api         string
	ingress     string
	web         string
	description string
}

func newUpdateContextCommand(cli *command.Cli) *updateContextCommand {
	return &updateContextCommand{cli: cli}
}

func (c *updateContextCommand) Name() string {
	return "update"
}

func (c *updateContextCommand) Description() string {
	return "Update a context"
}

func (c *updateContextCommand) Usage() string {
	return command.Name + " context update [OPTIONS] CONTEXT"
}

func (c *updateContextCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.api, console.Long("api"), "The estate's API, as an absolute http or https URL")
	console.Var(flags, &c.ingress, console.Long("ingress"), "The estate's workload ingress, as an absolute http or https URL")
	console.Var(flags, &c.web, console.Long("web"), "The estate's website, as an absolute http or https URL")
	console.Var(flags, &c.description, console.Long("description"), "Description of the context")
}

func (c *updateContextCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *updateContextCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	name := c.flags.Arg(0)

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	updated, err := cfg.Context(name)
	if err != nil {
		return err
	}

	moved := c.provided("api") && c.api != updated.API
	if moved {
		updated.API = c.api
		updated.OAuthClient = nil
	}

	if c.provided("ingress") {
		updated.Ingress = c.ingress
	}

	if c.provided("web") {
		updated.Web = c.web
	}

	if c.provided("description") {
		updated.Description = c.description
	}

	if err := updated.Validate(); err != nil {
		return err
	}

	if moved {
		manager, err := c.cli.Auth()
		if err != nil {
			return err
		}

		if err := manager.Logout(ctx, name); err != nil {
			return err
		}
	}

	if err := cfg.SetContext(updated); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(c.cli.Out, name); err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.cli.Err, "Successfully updated context %q\n", name)

	return err
}

// provided reports whether the flag of that long name was given.
func (c *updateContextCommand) provided(name string) bool {
	flag := c.flags.Lookup(name)

	return flag != nil && flag.Provided()
}
