package system

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/config"
)

// createContextCommand is docker's context create: an estate the CLI can
// talk to, by a name of its own.
type createContextCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	api         string
	ingress     string
	web         string
	description string
}

func newCreateContextCommand(cli *command.Cli) *createContextCommand {
	return &createContextCommand{cli: cli}
}

func (c *createContextCommand) Name() string {
	return "create"
}

func (c *createContextCommand) Description() string {
	return "Create a context"
}

func (c *createContextCommand) Usage() string {
	return command.Name + " context create [OPTIONS] CONTEXT"
}

func (c *createContextCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.api, console.Long("api"), "The estate's API, as an absolute http or https URL (required)")
	console.Var(flags, &c.ingress, console.Long("ingress"), "The estate's workload ingress, as an absolute http or https URL")
	console.Var(flags, &c.web, console.Long("web"), "The estate's website, as an absolute http or https URL")
	console.Var(flags, &c.description, console.Long("description"), "Description of the context")
}

func (c *createContextCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createContextCommand) run(context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	if len(c.api) == 0 {
		return command.UsageError{Command: c.flags.Name(), Message: "--api is required"}
	}

	name := c.flags.Arg(0)

	cfg, err := c.cli.Config()
	if err != nil {
		return err
	}

	if _, err := cfg.Context(name); err == nil {
		return fmt.Errorf("context %q already exists", name)
	}

	if err := cfg.SetContext(config.Context{Name: name, Description: c.description, API: c.api, Ingress: c.ingress, Web: c.web}); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(c.cli.Out, name); err != nil {
		return err
	}

	_, err = fmt.Fprintf(c.cli.Err, "Successfully created context %q\n", name)

	return err
}
