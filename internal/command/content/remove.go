package content

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// removeCommand deletes one or more of a resource's things, writing the key
// of each it deleted, as docker's rm does.
type removeCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	flags    *console.FlagSet
	allUsers bool
}

var _ console.Command = &removeCommand{}

func newRemoveCommand(cli *command.Cli, d resource.Definition, group string) *removeCommand {
	return &removeCommand{cli: cli, resource: d, group: group}
}

func (c *removeCommand) Name() string {
	return "rm"
}

func (c *removeCommand) Description() string {
	return "Remove one or more " + plural(c.resource)
}

func (c *removeCommand) Usage() string {
	key := c.resource.KeyUsage()

	return c.group + " rm [OPTIONS] " + key + " [" + key + "...]"
}

func (c *removeCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if c.resource.ServesBoth(resource.OperationDelete) {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "removes them among everybody's "+plural(c.resource)+" rather than one's own.", console.Short("A"), console.Default(false))
	}
}

func (c *removeCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *removeCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	keys, err := parseKeys(c.flags, c.resource)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	path, _, _ := c.resource.Route(resource.OperationDelete, c.allUsers)

	var errs []error

	for _, key := range keys {
		body := c.resource.BodyKey(path, key)
		if len(body) > 0 {
			body = withDefaults(body, c.resource.Defaults)
		}

		if err := client.Do(ctx, http.MethodDelete, c.resource.Path(path, key), nil, requestBody(body), nil); err != nil {
			errs = append(errs, failed(c.resource, key, err))

			continue
		}

		if _, err := fmt.Fprintln(c.cli.Out, strings.Join(key, "/")); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}
