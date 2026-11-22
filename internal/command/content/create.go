package content

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// createCommand creates one of a resource's things from a JSON file, as
// the API takes it, and writes the key it was given.
type createCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	flags *console.FlagSet
	file  string
}

var _ console.Command = &createCommand{}

func newCreateCommand(cli *command.Cli, d resource.Definition, group string) *createCommand {
	return &createCommand{cli: cli, resource: d, group: group}
}

func (c *createCommand) Name() string {
	return "create"
}

func (c *createCommand) Description() string {
	return "Create " + indefinite(c.resource) + " from a JSON file"
}

func (c *createCommand) Usage() string {
	return c.group + " create -f FILE"
}

func (c *createCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.file, console.Long("file"), "the JSON file to create it from, as the API takes it; - reads the standard input.", console.Short("f"), console.Default(""))
}

func (c *createCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *createCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	if len(c.file) == 0 {
		return usageError(c.flags, "%q requires a file: -f FILE, or -f - for the standard input.", c.flags.Name())
	}

	body, err := readObject(c.cli, c.file)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	path, _, _ := c.resource.Route(resource.OperationCreate, false)

	var answer json.RawMessage
	if err := client.Post(ctx, path, withDefaults(body, c.resource.Defaults), &answer); err != nil {
		return err
	}

	if key := identify(c.resource, answer); len(key) > 0 {
		_, err = fmt.Fprintln(c.cli.Out, key)
	}

	return err
}
