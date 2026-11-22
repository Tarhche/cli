package content

import (
	"context"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// updateCommand changes one of a resource's things to what a JSON file
// says, as the API takes it, and writes its key. The key is the file's, or
// the one given as an argument, which takes its place.
type updateCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	flags    *console.FlagSet
	allUsers bool
	file     string
}

var _ console.Command = &updateCommand{}

func newUpdateCommand(cli *command.Cli, d resource.Definition, group string) *updateCommand {
	return &updateCommand{cli: cli, resource: d, group: group}
}

func (c *updateCommand) Name() string {
	return "update"
}

func (c *updateCommand) Description() string {
	return "Update " + indefinite(c.resource) + " from a JSON file"
}

func (c *updateCommand) Usage() string {
	usage := c.group + " update "
	if c.resource.ServesBoth(resource.OperationUpdate) {
		usage += "[OPTIONS] "
	}

	return usage + "-f FILE [" + c.resource.KeyUsage() + "]"
}

func (c *updateCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if c.resource.ServesBoth(resource.OperationUpdate) {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "updates it among everybody's "+plural(c.resource)+" rather than one's own.", console.Short("A"), console.Default(false))
	}

	console.Var(flags, &c.file, console.Long("file"), "the JSON file to update it from, the whole of it as the API takes it; - reads the standard input.", console.Short("f"), console.Default(""))
}

func (c *updateCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *updateCommand) run(ctx context.Context) error {
	if err := command.MaxArgs(c.flags, 1); err != nil {
		return err
	}

	if len(c.file) == 0 {
		return usageError(c.flags, "%q requires a file: -f FILE, or -f - for the standard input.", c.flags.Name())
	}

	body, err := readObject(c.cli, c.file)
	if err != nil {
		return err
	}

	path, _, _ := c.resource.Route(resource.OperationUpdate, c.allUsers)

	key, err := c.key(path, body)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	if err := client.Put(ctx, c.resource.Path(path, key), withDefaults(body, c.resource.Defaults), nil); err != nil {
		return failed(c.resource, key, err)
	}

	_, err = fmt.Fprintln(c.cli.Out, strings.Join(key, "/"))

	return err
}

// key is the key of what is updated: the argument, whose fields the body
// then says, or the body's own.
func (c *updateCommand) key(path string, body map[string]any) ([]string, error) {
	if c.flags.NArg() == 1 {
		key, err := c.resource.ParseKey(c.flags.Arg(0))
		if err != nil {
			return nil, usageError(c.flags, "%s", err)
		}

		for field, value := range c.resource.BodyKey(path, key) {
			body[field] = value
		}

		return key, nil
	}

	key := make([]string, len(c.resource.Key))

	for i, field := range c.resource.Key {
		value, isString := body[field].(string)
		if !isString || len(value) == 0 {
			return nil, usageError(c.flags, "the file says no %s: give the %s as an argument.", field, c.resource.KeyUsage())
		}

		key[i] = value
	}

	return key, nil
}
