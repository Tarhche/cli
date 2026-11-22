package content

import (
	"context"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
)

// userSubscriptionRoute is where the subscription a user is on is set.
const userSubscriptionRoute = "/api/dashboard/users/subscription"

// overTable is the table of what a subscription would leave past its limits.
const overTable = "{{.kind}}\t{{.uuid}}\t{{.name}}\t{{.reason}}\t{{.action}}\t{{ago .due_at}}"

// setSubscriptionCommand writes the subscription a user is on, or puts them
// on one, or on none; with --dry-run it writes what that would leave past
// the limits instead, and puts them on nothing.
type setSubscriptionCommand struct {
	cli   *command.Cli
	group string

	flags  *console.FlagSet
	none   bool
	dryRun bool
	format string
}

var _ console.Command = &setSubscriptionCommand{}

func newSetSubscriptionCommand(cli *command.Cli, group string) *setSubscriptionCommand {
	return &setSubscriptionCommand{cli: cli, group: group}
}

func (c *setSubscriptionCommand) Name() string {
	return "subscription"
}

func (c *setSubscriptionCommand) Description() string {
	return "Display or set the subscription a user is on"
}

func (c *setSubscriptionCommand) Usage() string {
	return c.group + " subscription [OPTIONS] UUID [SUBSCRIPTION_UUID]"
}

func (c *setSubscriptionCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.none, console.Long("none"), "puts the user on no subscription, which holds them to the platform's limits alone.", console.Default(false))
	console.Var(flags, &c.dryRun, console.Long("dry-run"), "writes what would be past the limits rather than putting the user on it.", console.Default(false))
	console.Var(flags, &c.format, console.Long("format"), `writes what would be past the limits as a table (the default), as JSON ("json"), or through a Go template.`, console.Default(""))
}

func (c *setSubscriptionCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *setSubscriptionCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	if err := command.MaxArgs(c.flags, 2); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	subscription := c.flags.Arg(1)

	switch {
	case len(subscription) > 0 && c.none:
		return usageError(c.flags, "give a subscription or --none, but not both.")
	case len(subscription) == 0 && !c.none && c.dryRun:
		return usageError(c.flags, "--dry-run requires a subscription, or --none.")
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	if len(subscription) == 0 && !c.none {
		var held struct {
			Subscription *struct {
				UUID string `json:"uuid"`
			} `json:"subscription"`
		}

		if err := client.Get(ctx, user.Path(user.Everyone.Get, key), nil, &held); err != nil {
			return failed(user, key, err)
		}

		if held.Subscription != nil {
			_, err = fmt.Fprintln(c.cli.Out, held.Subscription.UUID)
		}

		return err
	}

	body := map[string]any{"uuid": key[0], "subscription_uuid": subscription, "dry_run": c.dryRun}

	if !c.dryRun {
		if err := client.Put(ctx, userSubscriptionRoute, body, nil); err != nil {
			return failed(user, key, err)
		}

		_, err = fmt.Fprintln(c.cli.Out, key[0])

		return err
	}

	var answer struct {
		Over []map[string]any `json:"over"`
	}

	if err := client.Put(ctx, userSubscriptionRoute, body, &answer); err != nil {
		return failed(user, key, err)
	}

	over := make([]any, len(answer.Over))
	for i, entry := range answer.Over {
		over[i] = entry
	}

	return formatter.Write(c.cli.Out, over, formatter.Options{
		Format:  strings.TrimSpace(c.format),
		Table:   overTable,
		Headers: map[string]string{"due_at": "DUE"},
	})
}
