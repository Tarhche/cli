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

func TestSetCommand(t *testing.T) {
	t.Parallel()

	set := func(cli *command.Cli) *setCommand {
		return newSetCommand(cli, "set", "Change the settings from a JSON file", "tarhche config set -f FILE", "/api/dashboard/config")
	}

	t.Run("to what the file says, writing nothing", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/config": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(set(h.Cli), "-f", fileOf(t, `{"default_language_code":"en","user_default_roles":["r1"]}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"PUT /api/dashboard/config"}, api.routes())
		assert.JSONEq(t, `{"default_language_code":"en","user_default_roles":["r1"]}`, api.requests()[0].Body)
		assert.Empty(t, h.Out.String())
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"PUT /api/dashboard/config": answered(http.StatusBadRequest, `{"errors":{"default_language_code":"invalid_value"}}`)})
		h := harness(t, api)

		status := h.Run(set(h.Cli), "-f", fileOf(t, `{}`))

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: default_language_code: invalid_value\n", h.Err.String())
	})

	t.Run("a file that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(set(h.Cli), "-f", filepath.Join(t.TempDir(), "missing.json"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading ")
	})

	t.Run("no file is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(set(h.Cli))

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "requires a file")
	})

	t.Run("arguments are a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(set(h.Cli), "-f", "x", "y"))
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		command := set(nil)

		assert.Equal(t, "set", command.Name())
		assert.Equal(t, "Change the settings from a JSON file", command.Description())
		assert.Equal(t, "tarhche config set -f FILE", command.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return set(cli) }, "-f", fileOf(t, `{}`))
	})
}
