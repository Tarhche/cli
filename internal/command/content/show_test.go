package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestShowCommand(t *testing.T) {
	t.Parallel()

	const profile = `{"uuid":"u1","name":"Mahdi","email":"m@example.com","username":"mahdi","language_code":"en"}`

	profiles := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/profile":   answer(profile),
			"GET /api/exam-attempts/a1":    answer(`{"uuid":"a1","questions":10,"answered":3}`),
			"GET /api/dashboard/overview":  answered(http.StatusOK, ""),
			"GET /api/dashboard/forbidden": answered(http.StatusForbidden, `{"message":"forbidden"}`),
		})
	}

	show := func(cli *command.Cli, path string) *showCommand {
		return newShowCommand(cli, "show", "Display it", "tarhche thing show [OPTIONS]", path)
	}

	t.Run("indented, its fields in the order they came", func(t *testing.T) {
		t.Parallel()

		api := profiles()
		h := harness(t, api)

		status := h.Run(show(h.Cli, "/api/dashboard/profile"))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/profile"}, api.routes())
		assert.Equal(t, "{\n    \"uuid\": \"u1\",\n    \"name\": \"Mahdi\",\n    \"email\": \"m@example.com\",\n    \"username\": \"mahdi\",\n    \"language_code\": \"en\"\n}\n", h.Out.String())
	})

	t.Run("as JSON on a line", func(t *testing.T) {
		t.Parallel()

		h := harness(t, profiles())

		status := h.Run(show(h.Cli, "/api/dashboard/profile"), "--format", "json")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, profile+"\n", h.Out.String())
	})

	t.Run("through a template", func(t *testing.T) {
		t.Parallel()

		h := harness(t, profiles())

		status := h.Run(show(h.Cli, "/api/dashboard/profile"), "-f", "{{.username}} <{{.email}}>")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "mahdi <m@example.com>\n", h.Out.String())
	})

	t.Run("what its route names by the uuid given", func(t *testing.T) {
		t.Parallel()

		api := profiles()
		h := harness(t, api)

		status := h.Run(show(h.Cli, "/api/exam-attempts/{uuid}"), "-f", "{{.answered}}/{{.questions}}", "a1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/exam-attempts/a1"}, api.routes())
		assert.Equal(t, "3/10\n", h.Out.String())
	})

	t.Run("an empty answer is null", func(t *testing.T) {
		t.Parallel()

		h := harness(t, profiles())

		status := h.Run(show(h.Cli, "/api/dashboard/overview"))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "null\n", h.Out.String())
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, profiles())

		status := h.Run(show(h.Cli, "/api/dashboard/forbidden"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	for name, test := range map[string]struct {
		path string
		args []string
	}{
		"an argument to what names no uuid": {path: "/api/dashboard/profile", args: []string{"u1"}},
		"no uuid to what names one":         {path: "/api/exam-attempts/{uuid}"},
		"a uuid it is not":                  {path: "/api/exam-attempts/{uuid}", args: []string{"a/b"}},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(nil)
			h := harness(t, api)

			assert.Equal(t, console.ExitUsageError, h.Run(show(h.Cli, test.path), test.args...))
			assert.Empty(t, api.routes())
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		command := show(nil, "/api/dashboard/profile")

		assert.Equal(t, "show", command.Name())
		assert.Equal(t, "Display it", command.Description())
		assert.Equal(t, "tarhche thing show [OPTIONS]", command.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return show(cli, "/api/dashboard/profile") })
	})
}
