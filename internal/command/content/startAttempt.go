package content

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// startAttemptRoute is where an attempt at an exam is started, or the one
// in progress gone on with.
const startAttemptRoute = "/api/exams/{uuid}/attempts"

// startAttemptCommand starts an attempt at an exam, or goes on with the one
// in progress, and writes its uuid.
type startAttemptCommand struct {
	cli   *command.Cli
	group string

	flags     *console.FlagSet
	firstName string
	lastName  string
}

var _ console.Command = &startAttemptCommand{}

func newStartAttemptCommand(cli *command.Cli, group string) *startAttemptCommand {
	return &startAttemptCommand{cli: cli, group: group}
}

func (c *startAttemptCommand) Name() string {
	return "start"
}

func (c *startAttemptCommand) Description() string {
	return "Start an attempt at an exam, or go on with the one in progress"
}

func (c *startAttemptCommand) Usage() string {
	return c.group + " start [OPTIONS] EXAM_UUID"
}

func (c *startAttemptCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.firstName, console.Long("first-name"), "the first name the license the attempt issues names.", console.Default(""))
	console.Var(flags, &c.lastName, console.Long("last-name"), "the last name the license the attempt issues names.", console.Default(""))
}

func (c *startAttemptCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *startAttemptCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	exam := lookup("exam")

	key, err := exam.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	var answer json.RawMessage
	if err := client.Post(ctx, exam.Path(startAttemptRoute, key), map[string]any{"first_name": c.firstName, "last_name": c.lastName}, &answer); err != nil {
		return failed(exam, key, err)
	}

	if attempt := identify(lookup("exam-attempt"), answer); len(attempt) > 0 {
		_, err = fmt.Fprintln(c.cli.Out, attempt)
	}

	return err
}
