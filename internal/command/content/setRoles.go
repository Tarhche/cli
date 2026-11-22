package content

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// userRolesRoute is where the roles given to a user are set.
const userRolesRoute = "/api/dashboard/users/roles"

// uuids is a flag given as many times as there are uuids, each adding one.
type uuids []string

func (u *uuids) String() string {
	return strings.Join(*u, ",")
}

func (u *uuids) Set(value string) error {
	*u = append(*u, value)

	return nil
}

// Type names the flag's value in the help.
func (u *uuids) Type() string {
	return "uuid"
}

// setRolesCommand writes the roles given to a user, or sets them: to those
// it is given, to those the user has with some added or taken away, or to
// none. A role a user holds through their subscription is not one given.
type setRolesCommand struct {
	cli   *command.Cli
	group string

	flags  *console.FlagSet
	add    uuids
	remove uuids
	clear  bool
}

var _ console.Command = &setRolesCommand{}

func newSetRolesCommand(cli *command.Cli, group string) *setRolesCommand {
	return &setRolesCommand{cli: cli, group: group}
}

func (c *setRolesCommand) Name() string {
	return "roles"
}

func (c *setRolesCommand) Description() string {
	return "Display or set the roles given to a user"
}

func (c *setRolesCommand) Usage() string {
	return c.group + " roles [OPTIONS] UUID [ROLE_UUID...]"
}

func (c *setRolesCommand) Configure(flags *console.FlagSet) {
	c.flags = flags
	c.add, c.remove = nil, nil

	flags.Var(&c.add, console.Long("add"), "gives the user a role besides those they have; given again for each role.")
	flags.Var(&c.remove, console.Long("rm"), "takes a role away from the user; given again for each role.")
	console.Var(flags, &c.clear, console.Long("clear"), "takes every role away from the user.", console.Default(false))
}

func (c *setRolesCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *setRolesCommand) run(ctx context.Context) error {
	if err := command.MinArgs(c.flags, 1); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	given := c.flags.Args()[1:]
	adjusting := len(c.add) > 0 || len(c.remove) > 0

	if modes := countTrue(len(given) > 0, adjusting, c.clear); modes > 1 {
		return usageError(c.flags, "give the roles, --add and --rm, or --clear, but only one of them.")
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var held struct {
		RoleUUIDs []string `json:"role_uuids"`
	}

	if len(given) == 0 && !c.clear {
		if err := client.Get(ctx, user.Path(user.Everyone.Get, key), nil, &held); err != nil {
			return failed(user, key, err)
		}
	}

	var roles []string

	switch {
	case len(given) > 0:
		roles = given
	case c.clear:
		roles = []string{}
	case adjusting:
		roles = slices.DeleteFunc(slices.Concat(held.RoleUUIDs, c.add), func(role string) bool { return slices.Contains(c.remove, role) })
		slices.Sort(roles)
		roles = slices.Compact(roles)
	default:
		for _, role := range held.RoleUUIDs {
			if _, err := fmt.Fprintln(c.cli.Out, role); err != nil {
				return err
			}
		}

		return nil
	}

	if err := client.Put(ctx, userRolesRoute, map[string]any{"uuid": key[0], "role_uuids": roles}, nil); err != nil {
		return failed(user, key, err)
	}

	_, err = fmt.Fprintln(c.cli.Out, key[0])

	return err
}

// countTrue is how many of conditions hold.
func countTrue(conditions ...bool) int {
	count := 0

	for _, condition := range conditions {
		if condition {
			count++
		}
	}

	return count
}
