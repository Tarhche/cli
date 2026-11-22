package content

import (
	"context"
	"errors"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// userSessionRoute is where one of a user's sessions is ended.
const userSessionRoute = "/api/dashboard/users/{uuid}/sessions/{session}"

// userSessionKey is what names one of a user's sessions: the user's uuid
// and the session's.
var userSessionKey = resource.Definition{Key: []string{"uuid", "session"}}

// signOutCommand signs a user out of sessions of theirs, writing each it
// ended, or out of every one but the session asking.
type signOutCommand struct {
	cli   *command.Cli
	group string

	flags *console.FlagSet
	all   bool
}

var _ console.Command = &signOutCommand{}

func newSignOutCommand(cli *command.Cli, group string) *signOutCommand {
	return &signOutCommand{cli: cli, group: group}
}

func (c *signOutCommand) Name() string {
	return "signout"
}

func (c *signOutCommand) Description() string {
	return "Sign a user out of sessions, or of every one but the one asking"
}

func (c *signOutCommand) Usage() string {
	return c.group + " signout [OPTIONS] UUID [SESSION_UUID...]"
}

func (c *signOutCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.all, console.Long("all"), "signs the user out of every session but the one asking.", console.Short("a"), console.Default(false))
}

func (c *signOutCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *signOutCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	sessions := c.flags.Args()[1:]
	if c.all == (len(sessions) > 0) {
		return usageError(c.flags, "give the sessions to sign the user out of, or --all, but not both.")
	}

	for _, session := range sessions {
		if _, err := uuidKey.ParseKey(session); err != nil {
			return usageError(c.flags, "%s", err)
		}
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	if c.all {
		if err := client.Delete(ctx, user.Path(userSessionsRoute, key), nil, nil); err != nil {
			return failed(user, key, err)
		}

		_, err = fmt.Fprintln(c.cli.Out, key[0])

		return err
	}

	var errs []error

	for _, session := range sessions {
		if err := client.Delete(ctx, userSessionKey.Path(userSessionRoute, []string{key[0], session}), nil, nil); err != nil {
			errs = append(errs, failed(lookup("session"), []string{session}, err))

			continue
		}

		if _, err := fmt.Fprintln(c.cli.Out, session); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}
