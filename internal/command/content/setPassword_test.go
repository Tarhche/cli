package content

import (
	"errors"
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

// answering is a prompter that answers every question with answers in
// turn, and keeps the questions.
func answering(questions *[]string, answers ...string) prompter {
	return func(question string) (string, error) {
		*questions = append(*questions, question)
		answer := answers[0]
		answers = answers[1:]

		return answer, nil
	}
}

func TestSetPasswordCommand(t *testing.T) {
	t.Parallel()

	passwords := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{"PUT " + setPasswordRoute: answered(http.StatusNoContent, "")})
	}

	t.Run("from the standard input, writing whose it is", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)
		h.In.Reset("s3cret\r\nignored\n")

		status := h.Run(newSetPasswordCommand(h.Cli, "tarhche user"), "--password-stdin", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/users/password"}, api.routes())
		assert.JSONEq(t, `{"uuid":"u1","new_password":"s3cret"}`, api.requests()[0].Body)
		assert.Equal(t, "u1\n", h.Out.String())
	})

	t.Run("typed at the terminal", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)

		var questions []string

		password := newSetPasswordCommand(h.Cli, "tarhche user")
		password.prompt = answering(&questions, "typed")

		status := h.Run(password, "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"New password: "}, questions)
		assert.JSONEq(t, `{"uuid":"u1","new_password":"typed"}`, api.requests()[0].Body)
	})

	t.Run("nobody at a terminal to type it", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)

		status := h.Run(newSetPasswordCommand(h.Cli, "tarhche user"), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "--password-stdin")
		assert.Empty(t, api.routes())
	})

	t.Run("a standard input that ends before it", func(t *testing.T) {
		t.Parallel()

		h := harness(t, passwords())

		status := h.Run(newSetPasswordCommand(h.Cli, "tarhche user"), "--password-stdin", "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "the standard input ends before its line 1")
	})

	t.Run("a user that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"PUT " + setPasswordRoute: answered(http.StatusNotFound, "")}))
		h.In.Reset("s3cret\n")

		status := h.Run(newSetPasswordCommand(h.Cli, "tarhche user"), "--password-stdin", "u9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such user: u9\n", h.Err.String())
	})

	for name, args := range map[string][]string{
		"no uuid":          nil,
		"two uuids":        {"u1", "u2"},
		"a uuid it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			h := harness(t, nil)

			assert.Equal(t, console.ExitUsageError, h.Run(newSetPasswordCommand(h.Cli, "tarhche user"), args...))
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		password := newSetPasswordCommand(nil, "tarhche user")

		assert.Equal(t, "password", password.Name())
		assert.Equal(t, "Set a user's password", password.Description())
		assert.Equal(t, "tarhche user password [OPTIONS] UUID", password.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			password := newSetPasswordCommand(cli, "tarhche user")
			password.prompt = func(string) (string, error) { return "typed", nil }

			return password
		}, "u1")
	})
}

func TestChangePasswordCommand(t *testing.T) {
	t.Parallel()

	passwords := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{"PUT " + changePasswordRoute: answered(http.StatusNoContent, "")})
	}

	t.Run("from the standard input, the current one and then the new one", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)
		h.In.Reset("old\nnew")

		status := h.Run(newChangePasswordCommand(h.Cli, "tarhche profile"), "--password-stdin")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/password"}, api.routes())
		assert.JSONEq(t, `{"current_password":"old","new_password":"new"}`, api.requests()[0].Body)
		assert.Empty(t, h.Out.String())
	})

	t.Run("typed at the terminal", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)

		var questions []string

		password := newChangePasswordCommand(h.Cli, "tarhche profile")
		password.prompt = answering(&questions, "old", "new")

		status := h.Run(password)

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"Current password: ", "New password: "}, questions)
		assert.JSONEq(t, `{"current_password":"old","new_password":"new"}`, api.requests()[0].Body)
	})

	t.Run("nobody at a terminal to type them", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)

		status := h.Run(newChangePasswordCommand(h.Cli, "tarhche profile"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "--password-stdin")
		assert.Empty(t, api.routes())
	})

	t.Run("no new one typed", func(t *testing.T) {
		t.Parallel()

		api := passwords()
		h := harness(t, api)

		asked := 0
		password := newChangePasswordCommand(h.Cli, "tarhche profile")
		password.prompt = func(string) (string, error) {
			if asked++; asked == 2 {
				return "", errors.New("interrupted")
			}

			return "old", nil
		}

		status := h.Run(password)

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: interrupted\n", h.Err.String())
		assert.Empty(t, api.routes())
	})

	t.Run("a standard input that ends before the new one", func(t *testing.T) {
		t.Parallel()

		h := harness(t, passwords())
		h.In.Reset("old\n")

		status := h.Run(newChangePasswordCommand(h.Cli, "tarhche profile"), "--password-stdin")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "the standard input ends before its line 2")
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"PUT " + changePasswordRoute: answered(http.StatusBadRequest, `{"errors":{"current_password":"invalid_value"}}`)}))
		h.In.Reset("old\nnew\n")

		status := h.Run(newChangePasswordCommand(h.Cli, "tarhche profile"), "--password-stdin")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: current_password: invalid_value\n", h.Err.String())
	})

	t.Run("arguments are a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newChangePasswordCommand(h.Cli, "tarhche profile"), "x"))
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		password := newChangePasswordCommand(nil, "tarhche profile")

		assert.Equal(t, "password", password.Name())
		assert.Equal(t, "Change one's password", password.Description())
		assert.Equal(t, "tarhche profile password [OPTIONS]", password.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			password := newChangePasswordCommand(cli, "tarhche profile")
			password.prompt = func(string) (string, error) { return "typed", nil }

			return password
		})
	})
}
