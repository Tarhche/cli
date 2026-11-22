package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestSignOutCommand(t *testing.T) {
	t.Parallel()

	sessions := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"DELETE /api/dashboard/users/u1/sessions/s1": answered(http.StatusNoContent, ""),
			"DELETE /api/dashboard/users/u1/sessions/s2": answered(http.StatusNoContent, ""),
			"DELETE /api/dashboard/users/u1/sessions":    answered(http.StatusNoContent, ""),
		})
	}

	t.Run("out of the sessions given, writing each", func(t *testing.T) {
		t.Parallel()

		api := sessions()
		h := harness(t, api)

		status := h.Run(newSignOutCommand(h.Cli, "tarhche user"), "u1", "s1", "s2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/users/u1/sessions/s1", "DELETE /api/dashboard/users/u1/sessions/s2"}, api.routes())
		assert.Equal(t, "s1\ns2\n", h.Out.String())
	})

	t.Run("out of every session but the one asking", func(t *testing.T) {
		t.Parallel()

		api := sessions()
		h := harness(t, api)

		status := h.Run(newSignOutCommand(h.Cli, "tarhche user"), "--all", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/users/u1/sessions"}, api.routes())
		assert.Equal(t, "u1\n", h.Out.String())
	})

	t.Run("what it signed out of, and an error for what it did not", func(t *testing.T) {
		t.Parallel()

		h := harness(t, sessions())

		status := h.Run(newSignOutCommand(h.Cli, "tarhche user"), "u1", "s9", "s1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "s1\n", h.Out.String())
		assert.Equal(t, "Error: no such session: s9\n", h.Err.String())
	})

	t.Run("a user that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, sessions())

		status := h.Run(newSignOutCommand(h.Cli, "tarhche user"), "-a", "u9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such user: u9\n", h.Err.String())
	})

	for name, args := range map[string][]string{
		"neither sessions nor --all": {"u1"},
		"both sessions and --all":    {"--all", "u1", "s1"},
		"no uuid":                    nil,
		"a uuid it is not":           {"a/b", "s1"},
		"a session it is not":        {"u1", "a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := sessions()
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newSignOutCommand(h.Cli, "tarhche user"), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		signOut := newSignOutCommand(nil, "tarhche user")

		assert.Equal(t, "signout", signOut.Name())
		assert.Equal(t, "Sign a user out of sessions, or of every one but the one asking", signOut.Description())
		assert.Equal(t, "tarhche user signout [OPTIONS] UUID [SESSION_UUID...]", signOut.Usage())
	})

	t.Run("an output that cannot be written", func(t *testing.T) {
		t.Parallel()

		unwritable(t, sessions(), func(cli *command.Cli) console.Command { return newSignOutCommand(cli, "tarhche user") }, "u1", "s1")
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newSignOutCommand(cli, "tarhche user") }, "--all", "u1")
	})
}
