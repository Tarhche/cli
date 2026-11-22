package content

import (
	"context"
	"encoding/json"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// userSessionsRoute is where a user's sessions are listed, a shadow session
// among its opener's.
const userSessionsRoute = "/api/dashboard/users/{uuid}/sessions"

// userSessionsCommand lists where a user is signed in, as the sessions of
// one's profile are listed.
type userSessionsCommand struct {
	cli   *command.Cli
	group string

	flags  *console.FlagSet
	quiet  bool
	format string
}

var _ console.Command = &userSessionsCommand{}

func newUserSessionsCommand(cli *command.Cli, group string) *userSessionsCommand {
	return &userSessionsCommand{cli: cli, group: group}
}

func (c *userSessionsCommand) Name() string {
	return "sessions"
}

func (c *userSessionsCommand) Description() string {
	return "List where a user is signed in"
}

func (c *userSessionsCommand) Usage() string {
	return c.group + " sessions [OPTIONS] UUID"
}

func (c *userSessionsCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.quiet, console.Long("quiet"), "only writes what identifies each.", console.Short("q"), console.Default(false))
	console.Var(flags, &c.format, console.Long("format"), `writes them as a table ("table", the default, or "table TEMPLATE"), as JSON ("json"), or through a Go template.`, console.Default(""))
}

func (c *userSessionsCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *userSessionsCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var listed struct {
		Items []json.RawMessage `json:"items"`
	}

	if err := client.Get(ctx, user.Path(userSessionsRoute, key), nil, &listed); err != nil {
		return failed(user, key, err)
	}

	items, err := decodeAll(listed.Items)
	if err != nil {
		return err
	}

	session := lookup("session")

	return formatter.Write(c.cli.Out, items, formatter.Options{
		Format:  c.format,
		Table:   session.Table,
		Quiet:   c.quiet,
		ID:      session.IDTemplate(),
		Headers: session.Headers,
	})
}
