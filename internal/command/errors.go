package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danceable/console"

	"github.com/Tarhche/cli/internal/auth"
)

// ExitInterrupted is the status of a command stopped by an interrupt.
const ExitInterrupted = 130

// StatusError is an error that exits with Code, saying Message when there is
// one: the status of a command run in a workload, say.
type StatusError struct {
	Code    int
	Message string
}

func (e StatusError) Error() string {
	if len(e.Message) > 0 {
		return e.Message
	}

	return fmt.Sprintf("exit status %d", e.Code)
}

// UsageError is a command asked for wrongly. It exits with
// console.ExitUsageError and points at the command's help.
type UsageError struct {
	// Command is the command's full name, as "tarhche container rm".
	Command string
	Message string
}

func (e UsageError) Error() string {
	return e.Message
}

// Exit reports err as the CLI reports errors, and is the status the command
// exits with: success for none, a usage error with a pointer to the help,
// the status a StatusError carries, and failure for anything else.
func (c *Cli) Exit(err error) console.ExitStatus {
	var (
		usage  UsageError
		status StatusError
	)

	switch {
	case err == nil:
		return console.ExitSuccess
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	case errors.As(err, &usage):
		fmt.Fprintf(c.Err, "%s: %s\nSee '%s --help'.\n", Name, usage.Message, usage.Command)

		return console.ExitUsageError
	case errors.As(err, &status):
		if len(status.Message) > 0 {
			fmt.Fprintln(c.Err, status.Message)
		}

		return status.Code
	case errors.Is(err, auth.ErrNotLoggedIn):
		fmt.Fprintf(c.Err, "Error: %s\n", c.notLoggedIn(err))

		return console.ExitFailure
	default:
		fmt.Fprintf(c.Err, "Error: %s\n", err)

		return console.ExitFailure
	}
}

func (c *Cli) notLoggedIn(err error) string {
	current, contextErr := c.Context()
	if contextErr != nil {
		return err.Error()
	}

	login := Name + " login"
	if current.Name != c.contextOfConfig() {
		login = Name + " --context " + current.Name + " login"
	}

	return fmt.Sprintf("%s to %q (%s): run '%s' first", strings.TrimSuffix(err.Error(), "."), current.Name, current.API, login)
}

func (c *Cli) contextOfConfig() string {
	cfg, err := c.Config()
	if err != nil {
		return ""
	}

	return cfg.Current()
}

// NoArgs is an error when a command that takes no arguments was given some.
func NoArgs(flags *console.FlagSet) error {
	if flags.NArg() == 0 {
		return nil
	}

	return UsageError{Command: flags.Name(), Message: fmt.Sprintf("%q accepts no arguments.", flags.Name())}
}

// ExactArgs is an error unless a command was given n arguments.
func ExactArgs(flags *console.FlagSet, n int) error {
	if flags.NArg() == n {
		return nil
	}

	return UsageError{Command: flags.Name(), Message: fmt.Sprintf("%q requires exactly %d %s.", flags.Name(), n, arguments(n))}
}

// MinArgs is an error unless a command was given n arguments at least.
func MinArgs(flags *console.FlagSet, n int) error {
	if flags.NArg() >= n {
		return nil
	}

	return UsageError{Command: flags.Name(), Message: fmt.Sprintf("%q requires at least %d %s.", flags.Name(), n, arguments(n))}
}

// MaxArgs is an error when a command was given more than n arguments.
func MaxArgs(flags *console.FlagSet, n int) error {
	if flags.NArg() <= n {
		return nil
	}

	return UsageError{Command: flags.Name(), Message: fmt.Sprintf("%q requires at most %d %s.", flags.Name(), n, arguments(n))}
}

func arguments(n int) string {
	if n == 1 {
		return "argument"
	}

	return "arguments"
}
