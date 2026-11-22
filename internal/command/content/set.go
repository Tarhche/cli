package content

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// setCommand changes one thing that is not one of many, one's profile or
// the estate's settings, to what a JSON file says, the whole of it as the
// API takes it.
type setCommand struct {
	cli         *command.Cli
	name        string
	description string
	usage       string
	path        string

	flags *console.FlagSet
	file  string
}

var _ console.Command = &setCommand{}

func newSetCommand(cli *command.Cli, name, description, usage, path string) *setCommand {
	return &setCommand{cli: cli, name: name, description: description, usage: usage, path: path}
}

func (c *setCommand) Name() string {
	return c.name
}

func (c *setCommand) Description() string {
	return c.description
}

func (c *setCommand) Usage() string {
	return c.usage
}

func (c *setCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.file, console.Long("file"), "the JSON file to change it to, the whole of it as the API takes it; - reads the standard input.", console.Short("f"), console.Default(""))
}

func (c *setCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *setCommand) run(ctx context.Context) error {
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

	return client.Put(ctx, c.path, body, nil)
}
