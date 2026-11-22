package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestRemoveCommand(t *testing.T) {
	t.Parallel()

	comments := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"DELETE /api/dashboard/my/comments/m1": answered(http.StatusNoContent, ""),
			"DELETE /api/dashboard/my/comments/m2": answered(http.StatusNoContent, ""),
			"DELETE /api/dashboard/comments/m3":    answered(http.StatusNoContent, ""),
		})
	}

	t.Run("one's own by default, writing each removed", func(t *testing.T) {
		t.Parallel()

		api := comments()
		h := harness(t, api)

		status := h.Run(newRemoveCommand(h.Cli, lookup("comment"), "tarhche comment"), "m1", "m2")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/my/comments/m1", "DELETE /api/dashboard/my/comments/m2"}, api.routes())
		assert.Empty(t, api.requests()[0].Body)
		assert.Equal(t, "m1\nm2\n", h.Out.String())
	})

	t.Run("everybody's when asked", func(t *testing.T) {
		t.Parallel()

		api := comments()
		h := harness(t, api)

		status := h.Run(newRemoveCommand(h.Cli, lookup("comment"), "tarhche comment"), "--all-users", "m3")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/comments/m3"}, api.routes())
	})

	t.Run("a key its route does not name is its body's, with the defaults", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"DELETE /api/dashboard/my/bookmarks": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newRemoveCommand(h.Cli, lookup("bookmark"), "tarhche bookmark"), "c1/en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"DELETE /api/dashboard/my/bookmarks"}, api.routes())
		assert.JSONEq(t, `{"object_uuid":"c1","language_code":"en","object_type":"article"}`, api.requests()[0].Body)
		assert.Equal(t, "c1/en\n", h.Out.String())
	})

	t.Run("what was removed, and an error for what was not", func(t *testing.T) {
		t.Parallel()

		api := comments()
		h := harness(t, api)

		status := h.Run(newRemoveCommand(h.Cli, lookup("comment"), "tarhche comment"), "m9", "m1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "m1\n", h.Out.String())
		assert.Equal(t, "Error: no such comment: m9\n", h.Err.String())
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		remove := newRemoveCommand(nil, lookup("bookmark"), "tarhche bookmark")

		assert.Equal(t, "rm", remove.Name())
		assert.Equal(t, "Remove one or more bookmarks", remove.Description())
		assert.Equal(t, "tarhche bookmark rm [OPTIONS] OBJECT_UUID/LANGUAGE_CODE [OBJECT_UUID/LANGUAGE_CODE...]", remove.Usage())
	})

	for name, args := range map[string][]string{
		"no key":          nil,
		"a key it is not": {"m1/x"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(nil)
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(newRemoveCommand(h.Cli, lookup("comment"), "tarhche comment"), args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("an output that cannot be written", func(t *testing.T) {
		t.Parallel()

		unwritable(t, comments(), func(cli *command.Cli) console.Command {
			return newRemoveCommand(cli, lookup("comment"), "tarhche comment")
		}, "m1")
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newRemoveCommand(cli, lookup("comment"), "tarhche comment")
		}, "m1")
	})
}
