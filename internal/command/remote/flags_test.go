package remote

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestList(t *testing.T) {
	t.Parallel()

	var given list

	require.NoError(t, given.Set("A=b"))
	require.NoError(t, given.Set("C"))

	assert.Equal(t, list{"A=b", "C"}, given)
	assert.Equal(t, "A=b,C", given.String())
	assert.Equal(t, "list", given.Type())
}

func TestEnvironment(t *testing.T) {
	t.Parallel()

	r := &remote{lookupEnv: func(name string) (string, bool) {
		if name == "HOME" {
			return "/home/me", true
		}

		return "", false
	}}

	t.Run("a variable with a value is as it is, one without takes its value here, or is left out", func(t *testing.T) {
		t.Parallel()

		env, err := r.environment(list{"A=b", "EMPTY=", "HOME", "UNSET", "WITH=an=equals"})

		require.NoError(t, err)
		assert.Equal(t, []string{"A=b", "EMPTY=", "HOME=/home/me", "WITH=an=equals"}, env)
	})

	t.Run("a variable without a name is an error", func(t *testing.T) {
		t.Parallel()

		for _, variable := range []string{"=x", ""} {
			_, err := r.environment(list{variable})
			assert.EqualError(t, err, "invalid environment variable: "+variable)
		}
	})
}

func TestExitStatus(t *testing.T) {
	t.Parallel()

	failed := errors.New("could not be waited for")

	assert.NoError(t, exitStatus(0, nil))
	assert.Equal(t, failed, exitStatus(-1, failed))
	assert.Equal(t, command.StatusError{Code: 3}, exitStatus(3, nil))
	assert.Equal(t, command.StatusError{Code: 255}, exitStatus(255, nil))
	assert.Equal(t, command.StatusError{Code: 255, Message: "the command was killed by a signal"}, exitStatus(-1, nil))
	assert.Equal(t, command.StatusError{Code: 255, Message: "the command exited with 256"}, exitStatus(256, nil))
}
