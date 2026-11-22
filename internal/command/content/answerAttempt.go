package content

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// answerAttemptRoute is where a page of one's attempt is answered.
const answerAttemptRoute = "/api/exam-attempts/{uuid}/answers"

// answerAttemptCommand answers a page of one's attempt, as a whole, from a
// JSON file ({"stage", "page", "answers": [{"question_uuid", "option_ids",
// "text", "code"}]}), and writes the attempt's uuid.
type answerAttemptCommand struct {
	cli   *command.Cli
	group string

	flags *console.FlagSet
	file  string
}

var _ console.Command = &answerAttemptCommand{}

func newAnswerAttemptCommand(cli *command.Cli, group string) *answerAttemptCommand {
	return &answerAttemptCommand{cli: cli, group: group}
}

func (c *answerAttemptCommand) Name() string {
	return "answer"
}

func (c *answerAttemptCommand) Description() string {
	return "Answer a page of one's attempt from a JSON file"
}

func (c *answerAttemptCommand) Usage() string {
	return c.group + " answer -f FILE UUID"
}

func (c *answerAttemptCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.file, console.Long("file"), `the page's answers: {"stage", "page", "answers": [{"question_uuid", "option_ids", "text", "code"}]}; - reads the standard input.`, console.Short("f"), console.Default(""))
}

func (c *answerAttemptCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *answerAttemptCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	attempt := lookup("exam-attempt")

	key, err := attempt.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	if len(c.file) == 0 {
		return usageError(c.flags, "%q requires a file: -f FILE, or -f - for the standard input.", c.flags.Name())
	}

	answers, err := readObject(c.cli, c.file)
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	if err := client.Post(ctx, attempt.Path(answerAttemptRoute, key), answers, nil); err != nil {
		return failed(attempt, key, err)
	}

	_, err = fmt.Fprintln(c.cli.Out, key[0])

	return err
}
