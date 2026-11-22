package content

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"

	"github.com/Tarhche/cli/internal/command"
)

// errNoTerminal is a password asked for with nobody at a terminal to type it.
var errNoTerminal = errors.New("no terminal to type a password at: give it on the standard input with --password-stdin")

// prompter asks for something that is not to be echoed.
type prompter func(question string) (string, error)

// terminalPrompter asks at the terminal the CLI reads from, writing the
// question where questions are asked.
func terminalPrompter(cli *command.Cli) prompter {
	return func(question string) (string, error) {
		terminal, isFile := cli.In.(interface{ Fd() uintptr })
		if !isFile || !cli.Interactive() {
			return "", errNoTerminal
		}

		_, _ = fmt.Fprint(cli.Err, question)
		secret, err := term.ReadPassword(int(terminal.Fd()))
		_, _ = fmt.Fprintln(cli.Err)

		return string(secret), err
	}
}

// readLines are the first n lines of in, without their line endings.
func readLines(in io.Reader, n int) ([]string, error) {
	reader := bufio.NewReader(in)
	lines := make([]string, n)

	for i := range lines {
		line, err := reader.ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || len(line) == 0) {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("the standard input ends before its line %d", i+1)
			}

			return nil, fmt.Errorf("reading the standard input: %w", err)
		}

		lines[i] = strings.TrimRight(line, "\r\n")
	}

	return lines, nil
}
