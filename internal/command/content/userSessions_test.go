package content

import (
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestUserSessionsCommand(t *testing.T) {
	t.Parallel()

	sessions := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/users/u1/sessions": answer(`{"items":[{"uuid":"s1","current":false,"device":{"browser":"Firefox","os":"Linux"},"ip":"10.0.0.1","created_at":"0001-01-01T00:00:00Z","last_activity":{"at":"0001-01-01T00:00:00Z"},"expires_at":"0001-01-01T00:00:00Z"}]}`),
		})
	}

	t.Run("where a user is signed in, as one's own sessions are listed", func(t *testing.T) {
		t.Parallel()

		api := sessions()
		h := harness(t, api)

		status := h.Run(newUserSessionsCommand(h.Cli, "tarhche user"), "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/users/u1/sessions"}, api.routes())
		assert.Equal(t, ""+
			"UUID      CURRENT   CLIENT    BROWSER   OS        AS        IP         LAST ACTIVE   CREATED\n"+
			"s1                            Firefox   Linux               10.0.0.1                 \n", h.Out.String())
	})

	t.Run("what identifies each", func(t *testing.T) {
		t.Parallel()

		h := harness(t, sessions())

		status := h.Run(newUserSessionsCommand(h.Cli, "tarhche user"), "-q", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "s1\n", h.Out.String())
	})

	t.Run("a user that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, sessions())

		status := h.Run(newUserSessionsCommand(h.Cli, "tarhche user"), "u9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such user: u9\n", h.Err.String())
	})

	for name, args := range map[string][]string{
		"no uuid":          nil,
		"a uuid it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			h := harness(t, nil)

			assert.Equal(t, console.ExitUsageError, h.Run(newUserSessionsCommand(h.Cli, "tarhche user"), args...))
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		list := newUserSessionsCommand(nil, "tarhche user")

		assert.Equal(t, "sessions", list.Name())
		assert.Equal(t, "List where a user is signed in", list.Description())
		assert.Equal(t, "tarhche user sessions [OPTIONS] UUID", list.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newUserSessionsCommand(cli, "tarhche user") }, "u1")
	})
}
