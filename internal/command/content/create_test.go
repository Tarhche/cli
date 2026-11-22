package content

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

// fileOf is a file holding content, in a directory of the test's own.
func fileOf(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "file.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestCreateCommand(t *testing.T) {
	t.Parallel()

	t.Run("from a file, as it is, writing the key it was given", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"POST /api/dashboard/articles": answered(http.StatusCreated, `{"correlation_uuid":"c1","language_code":"en"}`),
		})
		h := harness(t, api)

		status := h.Run(newCreateCommand(h.Cli, lookup("article"), "tarhche article"), "-f", fileOf(t, `{"title":"Hello","view_count":31536000,"language_code":"en"}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		require.Len(t, api.requests(), 1)
		assert.Equal(t, "POST /api/dashboard/articles", api.routes()[0])
		assert.JSONEq(t, `{"title":"Hello","view_count":31536000,"language_code":"en"}`, api.requests()[0].Body)
		assert.Equal(t, "application/json", api.requests()[0].Header.Get("Content-Type"))
		assert.Equal(t, "c1/en\n", h.Out.String())
	})

	t.Run("with the defaults of what the file does not say, writing nothing of an answer without a key", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/dashboard/comments": answered(http.StatusCreated, "")})
		h := harness(t, api)

		status := h.Run(newCreateCommand(h.Cli, lookup("comment"), "tarhche comment"), "--file", fileOf(t, `{"body":"Hi","object_uuid":"c1"}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"body":"Hi","object_uuid":"c1","object_type":"article"}`, api.requests()[0].Body)
		assert.Empty(t, h.Out.String())
	})

	t.Run("a default does not take the place of what the file says", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/dashboard/comments": answered(http.StatusCreated, "")})
		h := harness(t, api)

		status := h.Run(newCreateCommand(h.Cli, lookup("comment"), "tarhche comment"), "-f", fileOf(t, `{"object_type":"video"}`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"object_type":"video"}`, api.requests()[0].Body)
	})

	t.Run("from the standard input", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/dashboard/languages": answered(http.StatusCreated, `{"code":"de"}`)})
		h := harness(t, api)
		h.In.Reset(`{"code":"de","name":"Deutsch"}`)

		status := h.Run(newCreateCommand(h.Cli, lookup("language"), "tarhche language"), "-f", "-")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"code":"de","name":"Deutsch"}`, api.requests()[0].Body)
		assert.Equal(t, "de\n", h.Out.String())
	})

	t.Run("from an array of one object, as inspect writes it", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/dashboard/roles": answered(http.StatusCreated, `{"uuid":"r1"}`)})
		h := harness(t, api)

		status := h.Run(newCreateCommand(h.Cli, lookup("role"), "tarhche role"), "-f", fileOf(t, `[{"translations":{"en":{"title":"Admin"}}}]`))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.JSONEq(t, `{"translations":{"en":{"title":"Admin"}}}`, api.requests()[0].Body)
		assert.Equal(t, "r1\n", h.Out.String())
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/dashboard/users": answered(http.StatusBadRequest, `{"errors":{"email":"required_field"}}`)})
		h := harness(t, api)

		status := h.Run(newCreateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", fileOf(t, `{}`))

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: email: required_field\n", h.Err.String())
	})

	for name, content := range map[string]string{
		"no JSON":            `{`,
		"no object":          `[1, 2]`,
		"nothing":            ``,
		"an array of others": `[{}, {}]`,
	} {
		t.Run("a file holding "+name, func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(nil)
			h := harness(t, api)

			status := h.Run(newCreateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", fileOf(t, content))

			assert.Equal(t, console.ExitFailure, status)
			assert.Contains(t, h.Err.String(), "file.json")
			assert.Empty(t, api.routes())
		})
	}

	t.Run("a file that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newCreateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", filepath.Join(t.TempDir(), "missing.json"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading ")
	})

	t.Run("no file is a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newCreateCommand(h.Cli, lookup("user"), "tarhche user"))

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "requires a file")
	})

	t.Run("arguments are a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newCreateCommand(h.Cli, lookup("user"), "tarhche user"), "-f", "x", "y"))
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		create := newCreateCommand(nil, lookup("exam"), "tarhche exam")

		assert.Equal(t, "create", create.Name())
		assert.Equal(t, "Create an exam from a JSON file", create.Description())
		assert.Equal(t, "tarhche exam create -f FILE", create.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newCreateCommand(cli, lookup("user"), "tarhche user")
		}, "-f", fileOf(t, `{}`))
	})
}
