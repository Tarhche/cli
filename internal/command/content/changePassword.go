package content

import (
	"context"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/command"
)

// changePasswordRoute is where whoever is signed in changes their password.
const changePasswordRoute = "/api/dashboard/password"

// changePasswordCommand changes one's own password, the current one and
// the new one typed at the terminal or read from the standard input.
type changePasswordCommand struct {
	cli    *command.Cli
	group  string
	prompt prompter

	flags         *console.FlagSet
	passwordStdin bool
}

var _ console.Command = &changePasswordCommand{}

func newChangePasswordCommand(cli *command.Cli, group string) *changePasswordCommand {
	return &changePasswordCommand{cli: cli, group: group, prompt: terminalPrompter(cli)}
}

func (c *changePasswordCommand) Name() string {
	return "password"
}

func (c *changePasswordCommand) Description() string {
	return "Change one's password"
}

func (c *changePasswordCommand) Usage() string {
	return c.group + " password [OPTIONS]"
}

func (c *changePasswordCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.passwordStdin, console.Long("password-stdin"), "reads the current password and then the new one from the standard input, a line each, rather than the terminal.", console.Default(false))
}

func (c *changePasswordCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *changePasswordCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	current, replacement, err := c.passwords()
	if err != nil {
		return err
	}

	client, err := c.cli.Client(ctx)
	if err != nil {
		return err
	}

	return client.Put(ctx, changePasswordRoute, map[string]any{"current_password": current, "new_password": replacement}, nil)
}

// passwords are the current password and the new one, from where they
// were asked to be read.
func (c *changePasswordCommand) passwords() (string, string, error) {
	if c.passwordStdin {
		lines, err := readLines(c.cli.In, 2)
		if err != nil {
			return "", "", err
		}

		return lines[0], lines[1], nil
	}

	current, err := c.prompt("Current password: ")
	if err != nil {
		return "", "", err
	}

	replacement, err := c.prompt("New password: ")
	if err != nil {
		return "", "", err
	}

	return current, replacement, nil
}
