package system

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/danceable/console"
	"golang.org/x/term"

	"github.com/Tarhche/cli/internal/command"
)

// loginCommand is docker's login: it signs the CLI in to the context it
// talks to, in a browser, or with a password when it is given a username.
type loginCommand struct {
	cli   *command.Cli
	flags *console.FlagSet

	username      string
	password      string
	passwordStdin bool

	// interactive reports whether somebody is at a terminal to be asked for
	// a password, and readPassword asks them without showing what they type.
	interactive  func() bool
	readPassword func() (string, error)
}

func newLoginCommand(cli *command.Cli) *loginCommand {
	c := &loginCommand{cli: cli, interactive: cli.Interactive}
	c.readPassword = c.readTerminalPassword

	return c
}

func (c *loginCommand) Name() string {
	return "login"
}

func (c *loginCommand) Description() string {
	return "Log in to the estate of the current context, in a browser or with a password"
}

func (c *loginCommand) Usage() string {
	return command.Name + " login [OPTIONS]"
}

func (c *loginCommand) Configure(flags *console.FlagSet) {
	c.flags = flags

	console.Var(flags, &c.username, console.Long("username"), "Username or email to log in with a password, rather than in a browser", console.Short("u"))
	console.Var(flags, &c.password, console.Long("password"), "Password", console.Short("p"))
	console.Var(flags, &c.passwordStdin, console.Long("password-stdin"), "Take the password from stdin")
}

func (c *loginCommand) Run(ctx context.Context) console.ExitStatus {
	return c.cli.Exit(c.run(ctx))
}

func (c *loginCommand) run(ctx context.Context) error {
	if err := command.NoArgs(c.flags); err != nil {
		return err
	}

	switch {
	case len(c.password) > 0 && c.passwordStdin:
		return command.UsageError{Command: c.flags.Name(), Message: "--password and --password-stdin are mutually exclusive"}
	case (len(c.password) > 0 || c.passwordStdin) && len(c.username) == 0:
		return command.UsageError{Command: c.flags.Name(), Message: "Must provide --username with --password or --password-stdin"}
	}

	name, err := c.cli.ContextName()
	if err != nil {
		return err
	}

	manager, err := c.cli.Auth()
	if err != nil {
		return err
	}

	if len(c.username) == 0 {
		err = manager.LoginWithBrowser(ctx, name)
	} else {
		var password string
		if password, err = c.passwordOf(); err == nil {
			err = manager.LoginWithPassword(ctx, name, c.username, password)
		}
	}

	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.cli.Out, "Login Succeeded")

	return err
}

// passwordOf is the password to log in with: the one given, the one on the
// input, or the one the user types at the terminal.
func (c *loginCommand) passwordOf() (string, error) {
	var password string

	switch {
	case c.passwordStdin:
		content, err := io.ReadAll(c.cli.In)
		if err != nil {
			return "", fmt.Errorf("reading the password from stdin: %w", err)
		}

		password = strings.TrimSuffix(strings.TrimSuffix(string(content), "\n"), "\r")
	case len(c.password) > 0:
		_, _ = fmt.Fprintln(c.cli.Err, "WARNING! Using --password via the CLI is insecure. Use --password-stdin.")

		password = c.password
	case !c.interactive():
		return "", errors.New("cannot perform an interactive login from a non TTY device")
	default:
		_, _ = fmt.Fprint(c.cli.Err, "Password: ")

		typed, err := c.readPassword()
		_, _ = fmt.Fprintln(c.cli.Err)

		if err != nil {
			return "", fmt.Errorf("reading the password: %w", err)
		}

		password = typed
	}

	if len(password) == 0 {
		return "", errors.New("password required")
	}

	return password, nil
}

// readTerminalPassword reads a line from the terminal the input is, without
// showing what is typed.
func (c *loginCommand) readTerminalPassword() (string, error) {
	terminal, isFile := c.cli.In.(interface{ Fd() uintptr })
	if !isFile {
		return "", errors.New("the input is not a terminal")
	}

	password, err := term.ReadPassword(int(terminal.Fd()))

	return string(password), err
}
