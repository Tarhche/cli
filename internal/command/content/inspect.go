package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// inspectCommand shows one or more of a resource's things as the API
// answers them, as docker's inspect does: an indented JSON array, or each
// through --format.
type inspectCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	flags    *console.FlagSet
	allUsers bool
	format   string
}

var _ console.Command = &inspectCommand{}

func newInspectCommand(cli *command.Cli, d resource.Definition, group string) *inspectCommand {
	return &inspectCommand{cli: cli, resource: d, group: group}
}

func (c *inspectCommand) Name() string {
	return "inspect"
}

func (c *inspectCommand) Description() string {
	return "Display detailed information on one or more " + plural(c.resource)
}

func (c *inspectCommand) Usage() string {
	key := c.resource.KeyUsage()

	return c.group + " inspect [OPTIONS] " + key + " [" + key + "...]"
}

func (c *inspectCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if c.resource.ServesBoth(resource.OperationGet) {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "reads them among everybody's "+plural(c.resource)+" rather than one's own.", console.Short("A"), console.Default(false))
	}

	console.Var(flags, &c.format, console.Long("format"), `writes each as JSON on a line ("json") or through a Go template, rather than as an indented array.`, console.Short("f"), console.Default(""))
}

func (c *inspectCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *inspectCommand) run(ctx context.Context) error {
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

	path, _, _ := c.resource.Route(resource.OperationGet, c.allUsers)

	var (
		found []json.RawMessage
		errs  []error
	)

	for _, key := range keys {
		var answer json.RawMessage
		if err := client.Get(ctx, c.resource.Path(path, key), nil, &answer); err != nil {
			errs = append(errs, failed(c.resource, key, err))

			continue
		}

		if len(bytes.TrimSpace(answer)) == 0 {
			answer = json.RawMessage("null")
		}

		found = append(found, answer)
	}

	if err := c.write(found); err != nil {
		return err
	}

	return errors.Join(errs...)
}

// write writes what was found as --format says.
func (c *inspectCommand) write(found []json.RawMessage) error {
	if len(strings.TrimSpace(c.format)) == 0 {
		if found == nil {
			found = []json.RawMessage{}
		}

		return formatter.WriteJSON(c.cli.Out, found)
	}

	for _, answer := range found {
		if err := writeOne(c.cli.Out, answer, c.format); err != nil {
			return err
		}
	}

	return nil
}
