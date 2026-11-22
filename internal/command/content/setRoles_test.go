package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestSetRolesCommand(t *testing.T) {
	t.Parallel()

	users := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/users/u1": answer(`{"uuid":"u1","role_uuids":["r2","r1"],"subscription":null}`),
			"PUT " + userRolesRoute:       answered(http.StatusNoContent, ""),
		})
	}

	t.Run("the roles a user was given, a line each", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/users/u1"}, api.routes())
		assert.Equal(t, "r2\nr1\n", h.Out.String())
	})

	t.Run("set to those given, writing whose they are", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "u1", "r3", "r4")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/users/roles"}, api.routes())
		assert.JSONEq(t, `{"uuid":"u1","role_uuids":["r3","r4"]}`, api.requests()[0].Body)
		assert.Equal(t, "u1\n", h.Out.String())
	})

	t.Run("set to none", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "--clear", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"uuid":"u1","role_uuids":[]}`, api.requests()[0].Body)
	})

	t.Run("some added and some taken away of those the user has", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "--add", "r3", "--add", "r1", "--rm", "r2", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/users/u1", "PUT /api/dashboard/users/roles"}, api.routes())
		assert.JSONEq(t, `{"uuid":"u1","role_uuids":["r1","r3"]}`, api.requests()[1].Body)
	})

	for name, args := range map[string][]string{
		"roles and --add":   {"--add", "r1", "u1", "r2"},
		"--clear and --rm":  {"--clear", "--rm", "r1", "u1"},
		"roles and --clear": {"--clear", "u1", "r2"},
		"no uuid":           nil,
		"a uuid it is not":  {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := users()
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newSetRolesCommand(h.Cli, "tarhche user"), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("a user that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, users())

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "u9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such user: u9\n", h.Err.String())
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"PUT " + userRolesRoute: answered(http.StatusBadRequest, `{"errors":{"role_uuids":"invalid_value"}}`)}))

		status := h.Run(newSetRolesCommand(h.Cli, "tarhche user"), "u1", "r9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: u1: role_uuids: invalid_value\n", h.Err.String())
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		roles := newSetRolesCommand(nil, "tarhche user")

		assert.Equal(t, "roles", roles.Name())
		assert.Equal(t, "Display or set the roles given to a user", roles.Description())
		assert.Equal(t, "tarhche user roles [OPTIONS] UUID [ROLE_UUID...]", roles.Usage())
	})

	t.Run("an output that cannot be written", func(t *testing.T) {
		t.Parallel()

		unwritable(t, users(), func(cli *command.Cli) console.Command { return newSetRolesCommand(cli, "tarhche user") }, "u1")
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newSetRolesCommand(cli, "tarhche user") }, "u1")
	})
}

func TestUUIDs(t *testing.T) {
	t.Parallel()

	var given uuids

	require.NoError(t, given.Set("r1"))
	require.NoError(t, given.Set("r2"))

	assert.Equal(t, uuids{"r1", "r2"}, given)
	assert.Equal(t, "r1,r2", given.String())
	assert.Equal(t, "uuid", given.Type())
}

func TestCountTrue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, countTrue())
	assert.Equal(t, 2, countTrue(true, false, true))
}
