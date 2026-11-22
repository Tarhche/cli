package content

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// listCommand lists a resource's things, one's own unless asked for
// everybody's, as docker's ls lists containers: every page of them, or
// one, narrowed by the filters the listing takes.
type listCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	name        string
	description string

	// audience is whose things it lists whatever it is asked, when it is
	// not for whoever asks to choose.
	audience resource.Audience

	filters *filterFlags

	flags    *console.FlagSet
	allUsers bool
	page     uint
	cursor   string
	quiet    bool
	format   string
}

var _ console.Command = &listCommand{}

// listOption shapes a list command that is not a group's ls.
type listOption func(*listCommand)

// named is a list command of another name and description.
func named(name, description string) listOption {
	return func(c *listCommand) {
		c.name, c.description = name, description
	}
}

// forAudience is a list command of the things of one audience alone.
func forAudience(audience resource.Audience) listOption {
	return func(c *listCommand) { c.audience = audience }
}

func newListCommand(cli *command.Cli, d resource.Definition, group string, options ...listOption) *listCommand {
	c := &listCommand{cli: cli, resource: d, group: group, name: "ls", description: "List " + plural(d)}
	for _, option := range options {
		option(c)
	}

	c.filters = newFilterFlags(d, c.audiences())

	return c
}

func (c *listCommand) Name() string {
	return c.name
}

func (c *listCommand) Description() string {
	return c.description
}

func (c *listCommand) Usage() string {
	return c.group + " " + c.name + " [OPTIONS]"
}

func (c *listCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if len(c.audiences()) > 1 {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "lists everybody's "+plural(c.resource)+" rather than one's own.", console.Short("A"), console.Default(false))
	}

	if len(c.resource.Cursor) > 0 {
		console.Var(flags, &c.cursor, console.Long(flagName(c.resource.Cursor)), "lists the page that follows the one that named it as its next.", console.Default(""))
	} else {
		console.Var(flags, &c.page, console.Long("page"), "lists only that page, the first being 1, rather than every page.", console.Default(0))
	}

	console.Var(flags, &c.quiet, console.Long("quiet"), "only writes what identifies each.", console.Short("q"), console.Default(false))
	console.Var(flags, &c.format, console.Long("format"), `writes them as a table ("table", the default, or "table TEMPLATE"), as JSON ("json"), or through a Go template.`, console.Default(""))

	c.filters.define(flags)
}

func (c *listCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *listCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	path, audience := c.route()

	query, err := c.filters.query(c.flags, audience)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	items, err := c.fetch(ctx, client, path, query)
	if err != nil {
		return err
	}

	return formatter.Write(c.cli.Out, c.resource.Rows(items), formatter.Options{
		Format:  c.format,
		Table:   c.resource.Table,
		Quiet:   c.quiet,
		ID:      c.resource.IDTemplate(),
		Headers: c.resource.Headers,
	})
}

// audiences are whose things it may list.
func (c *listCommand) audiences() []resource.Audience {
	if len(c.audience) > 0 {
		return []resource.Audience{c.audience}
	}

	return slices.DeleteFunc(c.resource.Audiences(), func(audience resource.Audience) bool {
		return len(c.resource.PathsOf(audience).List) == 0
	})
}

// route is where the listing is, and whose things it lists.
func (c *listCommand) route() (string, resource.Audience) {
	if len(c.audience) > 0 {
		return c.resource.PathsOf(c.audience).List, c.audience
	}

	path, audience, _ := c.resource.Route(resource.OperationList, c.allUsers)

	return path, audience
}

// fetch are the items of the listing at path: one page of a listing paged
// by a cursor, the page asked for, or every page.
func (c *listCommand) fetch(ctx context.Context, client *api.Client, path string, query url.Values) ([]any, error) {
	var raws []json.RawMessage

	switch {
	case len(c.resource.Cursor) > 0:
		if len(c.cursor) > 0 {
			query.Set(c.resource.Cursor, c.cursor)
		}

		var page struct {
			Items []json.RawMessage `json:"items"`
			Next  string            `json:"next"`
		}

		if err := client.Get(ctx, path, query, &page); err != nil {
			return nil, err
		}

		if len(page.Next) > 0 {
			_, _ = fmt.Fprintf(c.cli.Err, "More are listed with --%s %s\n", flagName(c.resource.Cursor), page.Next)
		}

		raws = page.Items
	case c.page > 0:
		page, err := api.List[json.RawMessage](ctx, client, path, query, c.page)
		if err != nil {
			return nil, err
		}

		raws = page.Items
	default:
		all, err := api.All[json.RawMessage](ctx, client, path, query)
		if err != nil {
			return nil, err
		}

		raws = all
	}

	return decodeAll(raws)
}
