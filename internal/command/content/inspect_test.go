package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestInspectCommand(t *testing.T) {
	t.Parallel()

	const article = `{"title":"Hello","view_count":31536000,"language_code":"en","correlation_uuid":"c1"}`

	articles := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/articles/c1/en": answer(article),
			"GET /api/dashboard/articles/c1/fa":    answer(`{"title":"سلام <b>","language_code":"fa","correlation_uuid":"c1"}`),
			"GET /api/dashboard/my/articles/c3/en": answered(http.StatusInternalServerError, ""),
			"GET /api/dashboard/my/articles/c4/en": answered(http.StatusOK, ""),
		})
	}

	t.Run("one's own by default, as an indented array, its fields in the order they came", func(t *testing.T) {
		t.Parallel()

		api := articles()
		h := harness(t, api)

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "c1/en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/articles/c1/en"}, api.routes())
		assert.Equal(t, "[\n    {\n        \"title\": \"Hello\",\n        \"view_count\": 31536000,\n        \"language_code\": \"en\",\n        \"correlation_uuid\": \"c1\"\n    }\n]\n", h.Out.String())
	})

	t.Run("everybody's when asked", func(t *testing.T) {
		t.Parallel()

		api := articles()
		h := harness(t, api)

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "-A", "--format", "{{.title}}", "c1/fa")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/articles/c1/fa"}, api.routes())
		assert.Equal(t, "سلام <b>\n", h.Out.String())
	})

	t.Run("each as JSON on a line", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "-f", "json", "c1/en", "c1/en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, article+"\n"+article+"\n", h.Out.String())
	})

	t.Run("what was found, and an error for what was not", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "c2/en", "c1/en", "c3/en")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Out.String(), `"correlation_uuid": "c1"`)
		assert.Contains(t, h.Err.String(), "no such article: c2/en")
		assert.Contains(t, h.Err.String(), "c3/en: internal server error")
	})

	t.Run("nothing found is an empty array", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "c2/en")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "[]\n", h.Out.String())
	})

	t.Run("an empty answer is null", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "c4/en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "[\n    null\n]\n", h.Out.String())
	})

	t.Run("a template that cannot be parsed", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), "-f", "{{", "c1/en")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "parsing the format")
	})

	t.Run("no choice of whose where one audience is served", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"GET /api/dashboard/languages/en": answer(`{"code":"en"}`)})
		h := harness(t, api)

		assert.Equal(t, console.ExitUsageError, h.Run(newInspectCommand(h.Cli, lookup("language"), "tarhche language"), "-A", "en"))
		assert.Equal(t, console.ExitSuccess, h.Run(newInspectCommand(h.Cli, lookup("language"), "tarhche language"), "en"))
		assert.Equal(t, []string{"GET /api/dashboard/languages/en"}, api.routes())
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		inspect := newInspectCommand(nil, lookup("article"), "tarhche article")

		assert.Equal(t, "inspect", inspect.Name())
		assert.Equal(t, "Display detailed information on one or more articles", inspect.Description())
		assert.Equal(t, "tarhche article inspect [OPTIONS] CORRELATION_UUID/LANGUAGE_CODE [CORRELATION_UUID/LANGUAGE_CODE...]", inspect.Usage())
	})

	for name, args := range map[string][]string{
		"no key":            nil,
		"a key it is not":   {"c1"},
		"a key of dot dots": {"../en"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			api := newFakeAPI(nil)
			h := harness(t, api)

			status := h.Run(newInspectCommand(h.Cli, lookup("article"), "tarhche article"), args...)

			assert.Equal(t, console.ExitUsageError, status)
			assert.Empty(t, api.routes())
		})
	}

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newInspectCommand(cli, lookup("article"), "tarhche article")
		}, "c1/en")
	})
}
