package content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// activityCommand counts the actions of the audit log a step at a time, one's
// own unless asked for everybody's: a row for each step, with a column for
// each of the actions taken most and one for the rest.
type activityCommand struct {
	cli      *command.Cli
	resource resource.Definition
	group    string

	// actions are its routes, everybody's and one's own.
	actions []resource.Action

	filters *filterFlags

	flags    *console.FlagSet
	allUsers bool
	format   string
}

var _ console.Command = &activityCommand{}

func newActivityCommand(cli *command.Cli, d resource.Definition, group string) *activityCommand {
	c := &activityCommand{cli: cli, resource: d, group: group}

	for _, action := range d.Actions {
		if action.Name == "activity" {
			c.actions = append(c.actions, action)
		}
	}

	audiences := make([]resource.Audience, len(c.actions))
	for i, action := range c.actions {
		audiences[i] = action.Audience()
	}

	c.filters = newFilterFlags(d, audiences)

	return c
}

func (c *activityCommand) Name() string {
	return "activity"
}

func (c *activityCommand) Description() string {
	return "Count the actions taken a step at a time, over the last hour unless --from and --to say otherwise"
}

func (c *activityCommand) Usage() string {
	return c.group + " activity [OPTIONS]"
}

func (c *activityCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	if len(c.actions) > 1 {
		console.Var(flags, &c.allUsers, console.Long("all-users"), "counts everybody's actions rather than one's own.", console.Short("A"), console.Default(false))
	}

	console.Var(flags, &c.format, console.Long("format"), `writes the steps as a table ("table", the default, or "table TEMPLATE"), the answer as JSON ("json"), or the steps through a Go template.`, console.Default(""))

	c.filters.define(flags)
}

func (c *activityCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *activityCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	action := c.chosen()

	query, err := c.filters.query(c.flags, action.Audience())
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var answer json.RawMessage
	if err := client.Get(ctx, action.Path, query, &answer); err != nil {
		return err
	}

	if strings.TrimSpace(c.format) == formatter.JSONFormat {
		return writeOne(c.cli.Out, answer, c.format)
	}

	return c.write(answer)
}

// chosen is the route asked: one's own unless everybody's are asked for.
func (c *activityCommand) chosen() resource.Action {
	for _, action := range c.actions {
		if action.Mine != c.allUsers {
			return action
		}
	}

	return c.actions[0]
}

// write writes the steps of the answer as --format says: a row each, its
// moment, its total, a column for each action taken most, and the rest.
func (c *activityCommand) write(answer json.RawMessage) error {
	var activity struct {
		Actions []string `json:"actions"`
		Points  []struct {
			At      string                 `json:"at"`
			Total   json.Number            `json:"total"`
			Actions map[string]json.Number `json:"actions"`
			Other   json.Number            `json:"other"`
		} `json:"points"`
	}

	if err := json.Unmarshal(answer, &activity); err != nil {
		return fmt.Errorf("reading the answer: %w", err)
	}

	columns := []string{"{{.at}}", "{{.total}}"}
	headers := map[string]string{}

	for i, action := range activity.Actions {
		field := fmt.Sprintf("action%d", i)
		columns = append(columns, "{{."+field+"}}")
		headers[field] = strings.ToUpper(action)
	}

	columns = append(columns, "{{.other}}")

	rows := make([]any, len(activity.Points))

	for i, point := range activity.Points {
		counts := make(map[string]any, len(point.Actions))
		for action, count := range point.Actions {
			counts[action] = count
		}

		row := map[string]any{"at": point.At, "total": point.Total, "other": point.Other, "actions": counts}

		for j, action := range activity.Actions {
			count, counted := point.Actions[action]
			if !counted {
				count = "0"
			}

			row[fmt.Sprintf("action%d", j)] = count
		}

		rows[i] = row
	}

	return formatter.Write(c.cli.Out, rows, formatter.Options{
		Format:  c.format,
		Table:   strings.Join(columns, "\t"),
		Headers: headers,
	})
}
