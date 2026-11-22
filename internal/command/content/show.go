package content

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// uuidKey is the key of what a route names by its uuid.
var uuidKey = resource.Definition{Key: []string{"uuid"}}

// showCommand shows one thing as the API answers it, as indented JSON or
// through --format: one's profile, the estate's settings, or, when its
// route names a uuid, the thing that uuid names.
type showCommand struct {
	cli         *command.Cli
	name        string
	description string
	usage       string
	path        string

	flags  *console.FlagSet
	format string
}

var _ console.Command = &showCommand{}

func newShowCommand(cli *command.Cli, name, description, usage, path string) *showCommand {
	return &showCommand{cli: cli, name: name, description: description, usage: usage, path: path}
}

func (c *showCommand) Name() string {
	return c.name
}

func (c *showCommand) Description() string {
	return c.description
}

func (c *showCommand) Usage() string {
	return c.usage
}

func (c *showCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.format, console.Long("format"), `writes it as JSON on a line ("json") or through a Go template, rather than indented.`, console.Short("f"), console.Default(""))
}

func (c *showCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *showCommand) run(ctx context.Context) error {
	path, err := c.route()
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var answer json.RawMessage
	if err := client.Get(ctx, path, nil, &answer); err != nil {
		return err
	}

	return writeOne(c.cli.Out, answer, c.format)
}

// route is where what is shown is: the path, with the uuid it was given
// in place of the one it names.
func (c *showCommand) route() (string, error) {
	if !strings.Contains(c.path, "{uuid}") {
		return c.path, command.NoArgs(c.flags)
	}

	if err := command.ExactArgs(c.flags, 1); err != nil {
		return "", err
	}

	key, err := uuidKey.ParseKey(c.flags.Arg(0))
	if err != nil {
		return "", usageError(c.flags, "%s", err)
	}

	return uuidKey.Path(c.path, key), nil
}
