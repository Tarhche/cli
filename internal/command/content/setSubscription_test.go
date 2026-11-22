package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestSetSubscriptionCommand(t *testing.T) {
	t.Parallel()

	const over = `{"over":[{"kind":"vm","uuid":"v1","name":"web","reason":"count","action":"stop","due_at":"2999-01-01T00:00:00Z"}]}`

	users := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/users/u1": answer(`{"uuid":"u1","role_uuids":[],"subscription":{"uuid":"s1"}}`),
			"GET /api/dashboard/users/u2": answer(`{"uuid":"u2","role_uuids":[],"subscription":null}`),
			"PUT " + userSubscriptionRoute: {
				{status: http.StatusNoContent},
			},
		})
	}

	dryRuns := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{"PUT " + userSubscriptionRoute: answer(over)})
	}

	t.Run("the subscription a user is on", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/users/u1"}, api.routes())
		assert.Equal(t, "s1\n", h.Out.String())
	})

	t.Run("nothing of a user on none", func(t *testing.T) {
		t.Parallel()

		h := harness(t, users())

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "u2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Empty(t, h.Out.String())
	})

	t.Run("put on one, writing whose it is", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "u1", "s2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/users/subscription"}, api.routes())
		assert.JSONEq(t, `{"uuid":"u1","subscription_uuid":"s2","dry_run":false}`, api.requests()[0].Body)
		assert.Equal(t, "u1\n", h.Out.String())
	})

	t.Run("put on none", func(t *testing.T) {
		t.Parallel()

		api := users()
		h := harness(t, api)

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "--none", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"uuid":"u1","subscription_uuid":"","dry_run":false}`, api.requests()[0].Body)
	})

	t.Run("what putting them on one would leave past the limits", func(t *testing.T) {
		t.Parallel()

		api := dryRuns()
		h := harness(t, api)

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "--dry-run", "u1", "s2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"uuid":"u1","subscription_uuid":"s2","dry_run":true}`, api.requests()[0].Body)
		assert.Contains(t, h.Out.String(), "KIND      UUID      NAME      REASON    ACTION    DUE\n")
		assert.Contains(t, h.Out.String(), "vm        v1        web       count     stop      in ")
	})

	t.Run("what it would leave past the limits, as JSON", func(t *testing.T) {
		t.Parallel()

		h := harness(t, dryRuns())

		status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), "--dry-run", "--format", "json", "--none", "u1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, `{"action":"stop","due_at":"2999-01-01T00:00:00Z","kind":"vm","name":"web","reason":"count","uuid":"v1"}`+"\n", h.Out.String())
	})

	for name, args := range map[string][]string{
		"a subscription and --none":        {"--none", "u1", "s1"},
		"--dry-run without a subscription": {"--dry-run", "u1"},
		"no uuid":                          nil,
		"three arguments":                  {"u1", "s1", "x"},
		"a uuid it is not":                 {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := users()
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), args...))
			assert.Empty(t, api.routes())
		})
	}

	for name, args := range map[string][]string{
		"reading":   {"u9"},
		"setting":   {"u9", "s1"},
		"a dry run": {"--dry-run", "u9", "s1"},
	} {
		t.Run("a user that is not there, "+name, func(t *testing.T) {
			t.Parallel()

			h := harness(t, newFakeAPI(nil))

			status := h.Run(newSetSubscriptionCommand(h.Cli, "tarhche user"), args...)

			assert.Equal(t, console.ExitFailure, status)
			assert.Equal(t, "Error: no such user: u9\n", h.Err.String())
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		subscription := newSetSubscriptionCommand(nil, "tarhche user")

		assert.Equal(t, "subscription", subscription.Name())
		assert.Equal(t, "Display or set the subscription a user is on", subscription.Description())
		assert.Equal(t, "tarhche user subscription [OPTIONS] UUID [SUBSCRIPTION_UUID]", subscription.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newSetSubscriptionCommand(cli, "tarhche user") }, "u1")
	})
}
