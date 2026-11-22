package content

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestUpdateCommand(t *testing.T) {
	t.Parallel()

	t.Run("the key the file says, writing it", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/users": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newUpdateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", fileOf(t, `{"uuid":"u1","name":"Sarah"}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/users"}, api.routes())
		assert.JSONEq(t, `{"uuid":"u1","name":"Sarah"}`, api.requests()[0].Body)
		assert.Equal(t, "u1\n", h.Out.String())
	})

	t.Run("one's own by default, the key given in the place of the file's", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/my/articles": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newUpdateCommand(h.Cli, lookup("article"), "tarhche article"), "-f", fileOf(t, `{"title":"Hello","language_code":"fa"}`), "c1/en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/my/articles"}, api.routes())
		assert.JSONEq(t, `{"title":"Hello","correlation_uuid":"c1","language_code":"en"}`, api.requests()[0].Body)
		assert.Equal(t, "c1/en\n", h.Out.String())
	})

	t.Run("everybody's when asked, with the defaults of what the file does not say", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/comments": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newUpdateCommand(h.Cli, lookup("comment"), "tarhche comment"), "-A", "-f", fileOf(t, `{"uuid":"m1","body":"Hi"}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/comments"}, api.routes())
		assert.JSONEq(t, `{"uuid":"m1","body":"Hi","object_type":"article"}`, api.requests()[0].Body)
	})

	t.Run("what is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(nil))

		status := h.Run(newUpdateCommand(h.Cli, lookup("article"), "tarhche article"), "-f", fileOf(t, `{"correlation_uuid":"c1","language_code":"en"}`))

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such article: c1/en\n", h.Err.String())
	})

	t.Run("a file that says no key, and no key given", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(nil)
		h := harness(t, api)

		status := h.Run(newUpdateCommand(h.Cli, lookup("article"), "tarhche article"), "-f", fileOf(t, `{"correlation_uuid":"c1","language_code":""}`))

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "the file says no language_code: give the CORRELATION_UUID/LANGUAGE_CODE as an argument.")
		assert.Empty(t, api.routes())
	})

	t.Run("a key it is not", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newUpdateCommand(h.Cli, lookup("article"), "tarhche article"), "-f", fileOf(t, `{}`), "c1")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), `invalid key "c1"`)
	})

	t.Run("more than one key is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newUpdateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", "x", "u1", "u2"))
	})

	t.Run("no file is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newUpdateCommand(h.Cli, lookup("user"), "tarhche user"), "u1")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "requires a file")
	})

	t.Run("a file that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newUpdateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", filepath.Join(t.TempDir(), "missing.json"), "u1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading ")
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		update := newUpdateCommand(nil, lookup("license-template"), "tarhche license-template")

		assert.Equal(t, "update", update.Name())
		assert.Equal(t, "Update a license template from a JSON file", update.Description())
		assert.Equal(t, "tarhche license-template update [OPTIONS] -f FILE [UUID]", update.Usage())
		assert.Equal(t, "tarhche user update -f FILE [UUID]", newUpdateCommand(nil, lookup("user"), "tarhche user").Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newUpdateCommand(cli, lookup("user"), "tarhche user")
		}, "-f", fileOf(t, `{}`), "u1")
	})
}
