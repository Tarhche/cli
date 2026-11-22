package content

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

// actCommand asks one of a resource's actions of the things it is given,
// writing the key of each it was carried out on; an action whose route
// names no key is asked once, of nothing in particular.
type actCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	// actions are the action's routes, everybody's and one's own.
	actions []resource.Action

	flags    *console.FlagSet
	allUsers bool
}

var _ console.Command = &actCommand{}

func newActCommand(cli *command.Cli, d resource.Definition, group string, actions []resource.Action) *actCommand {
	return &actCommand{cli: cli, resource: d, group: group, actions: actions}
}

func (c *actCommand) Name() string {
	return c.actions[0].Name
}

func (c *actCommand) Description() string {
	return c.actions[0].Description
}

func (c *actCommand) Usage() string {
	usage := c.group + " " + c.Name()
	if c.both() {
		usage += " [OPTIONS]"
	}

	if c.keyed(c.actions[0]) {
		key := c.resource.KeyUsage()
		usage += " " + key + " [" + key + "...]"
	}

	return usage
}

func (c *actCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if c.both() {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "asks it of everybody's "+plural(c.resource)+" rather than one's own.", console.Short("A"), console.Default(false))
	}
}

func (c *actCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *actCommand) run(ctx context.Context) error {
	action := c.chosen()

	if !c.keyed(action) {
		if err := command.NoArgs(c.flags); err != nil {
			return err
		}

		client, err := c.cli.Client(ctx)
		if err != nil {
			return err
		}

		return client.Do(ctx, action.Method, action.Path, nil, requestBody(action.Body), nil)
	}

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

	var errs []error

	for _, key := range keys {
		body := maps.Clone(action.Body)
		if body == nil {
			body = make(map[string]any)
		}

		maps.Copy(body, c.resource.BodyKey(action.Path, key))

		if err := client.Do(ctx, action.Method, c.resource.Path(action.Path, key), nil, requestBody(body), nil); err != nil {
			errs = append(errs, failed(c.resource, key, err))

			continue
		}

		if _, err := fmt.Fprintln(c.cli.Out, strings.Join(key, "/")); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}

// both reports whether the action is served for everybody's things and for
// one's own, so that whoever asks chooses whose.
func (c *actCommand) both() bool {
	return slices.ContainsFunc(c.actions, func(a resource.Action) bool { return a.Mine }) &&
		slices.ContainsFunc(c.actions, func(a resource.Action) bool { return !a.Mine })
}

// chosen is the route asked: one's own unless everybody's are asked for or
// one's own are not served.
func (c *actCommand) chosen() resource.Action {
	for _, action := range c.actions {
		if action.Mine != c.allUsers {
			return action
		}
	}

	return c.actions[0]
}

// keyed reports whether an action is asked of things of the resource,
// which its route names the key of.
func (c *actCommand) keyed(action resource.Action) bool {
	return len(c.resource.BodyKey(action.Path, c.resource.Key)) < len(c.resource.Key)
}
