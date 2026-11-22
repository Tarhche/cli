package content

import (
	"context"
	"fmt"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// setPasswordRoute is where a user's password is set by whoever may.
const setPasswordRoute = "/api/dashboard/users/password"

// setPasswordCommand sets a user's password, typed at the terminal or read
// from the standard input, and writes the user's uuid.
type setPasswordCommand struct {
	cli    *command.Cli
	group  string
	prompt prompter

	flags         *console.FlagSet
	passwordStdin bool
}

var _ console.Command = &setPasswordCommand{}

func newSetPasswordCommand(cli *command.Cli, group string) *setPasswordCommand {
	return &setPasswordCommand{cli: cli, group: group, prompt: terminalPrompter(cli)}
}

func (c *setPasswordCommand) Name() string {
	return "password"
}

func (c *setPasswordCommand) Description() string {
	return "Set a user's password"
}

func (c *setPasswordCommand) Usage() string {
	return c.group + " password [OPTIONS] UUID"
}

func (c *setPasswordCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.passwordStdin, console.Long("password-stdin"), "reads the new password from the first line of the standard input rather than the terminal.", console.Default(false))
}

func (c *setPasswordCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *setPasswordCommand) run(ctx context.Context) error {
	if err := command.ExactArgs(c.flags, 1); err != nil {
		return err
	}

	user := lookup("user")

	key, err := user.ParseKey(c.flags.Arg(0))
	if err != nil {
		return usageError(c.flags, "%s", err)
	}

	password, err := c.password()
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	if err := client.Put(ctx, setPasswordRoute, map[string]any{"uuid": key[0], "new_password": password}, nil); err != nil {
		return failed(user, key, err)
	}

	_, err = fmt.Fprintln(c.cli.Out, key[0])

	return err
}

// password is the new password, from where it was asked to be read.
func (c *setPasswordCommand) password() (string, error) {
	if !c.passwordStdin {
		return c.prompt("New password: ")
	}

	lines, err := readLines(c.cli.In, 1)
	if err != nil {
		return "", err
	}

	return lines[0], nil
}
