package remote

import (
	"fmt"
	"strings"

	"github.com/Tarhche/cli/internal/command"
)

// endOfTransmission is what a terminal reads as the end of its input, typed
// at the start of a line: what a terminal is sent once input that is not a
// terminal ended.
const endOfTransmission = 0x04

// list is a flag that is given any number of times, each a value of its
// own, as docker's -e is.
type list []string

func (l *list) String() string {
	return strings.Join(*l, ",")
}

func (l *list) Set(value string) error {
	*l = append(*l, value)

	return nil
}

func (l *list) Type() string {
	return "list"
}

// environment is the environment -e gave, as docker takes it: NAME=value as
// it is, and NAME alone with its value here, or left out when it has none.
func (r *remote) environment(given list) ([]string, error) {
	env := make([]string, 0, len(given))

	for _, variable := range given {
		name, _, hasValue := strings.Cut(variable, "=")

		switch {
		case len(name) == 0:
			return nil, fmt.Errorf("invalid environment variable: %s", variable)
		case hasValue:
			env = append(env, variable)
		default:
			if value, found := r.lookupEnv(name); found {
				env = append(env, name+"="+value)
			}
		}
	}

	return env, nil
}

// exitStatus is what a command run in a workload exiting with code, or not
// being waited for, makes the CLI exit with: its status.
func exitStatus(code int, err error) error {
	switch {
	case err != nil:
		return err
	case code == 0:
		return nil
	case code < 0:
		return command.StatusError{Code: 255, Message: "the command was killed by a signal"}
	case code > 255:
		return command.StatusError{Code: 255, Message: fmt.Sprintf("the command exited with %d", code)}
	}

	return command.StatusError{Code: code}
}
