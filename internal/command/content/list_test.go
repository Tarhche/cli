package content

import (
	"net/http"
	"testing"
	"time"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
	"github.com/Tarhche/cli/internal/resource"
)

const (
	articlesPage1 = `{"items":[{"correlation_uuid":"c1","corrolated_items":[
		{"title":"Hello","published_at":"0001-01-01T00:00:00Z","author":{"uuid":"u1","name":"Mahdi","avatar":"","username":"mahdi"},"language":{"code":"en","name":"English"}},
		{"title":"سلام","published_at":"0001-01-01T00:00:00Z","author":{"uuid":"u1","name":"Mahdi","avatar":"","username":"mahdi"},"language":{"code":"fa","name":"Persian"}}]}],
		"pagination":{"total_pages":2,"current_page":1}}`
	articlesPage2 = `{"items":[{"correlation_uuid":"c2","corrolated_items":[
		{"title":"Second","published_at":"0001-01-01T00:00:00Z","author":{"uuid":"u2","name":"Sara","avatar":"","username":"sara"},"language":{"code":"en","name":"English"}}]}],
		"pagination":{"total_pages":2,"current_page":2}}`
)

func TestListCommand(t *testing.T) {
	t.Parallel()

	articles := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/articles?page=1": answer(articlesPage1),
			"GET /api/dashboard/my/articles?page=2": answer(articlesPage2),
			"GET /api/dashboard/articles?page=1":    answer(`{"items":[],"pagination":{"total_pages":1,"current_page":1}}`),
		})
	}

	t.Run("one's own by default, every page of them, a row for each version", func(t *testing.T) {
		t.Parallel()

		api := articles()
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/articles?page=1", "GET /api/dashboard/my/articles?page=2"}, api.routes())
		assert.Equal(t, ""+
			"CORRELATION UUID   LANGUAGE   TITLE     AUTHOR    PUBLISHED\n"+
			"c1                 en         Hello     Mahdi     \n"+
			"c1                 fa         سلام      Mahdi     \n"+
			"c2                 en         Second    Sara      \n", h.Out.String())
	})

	t.Run("everybody's when asked", func(t *testing.T) {
		t.Parallel()

		api := articles()
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "-A")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/articles?page=1"}, api.routes())
		assert.Equal(t, "CORRELATION UUID   LANGUAGE   TITLE     AUTHOR    PUBLISHED\n", h.Out.String())
	})

	t.Run("the page asked for alone", func(t *testing.T) {
		t.Parallel()

		api := articles()
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "--page", "2", "-q")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/articles?page=2"}, api.routes())
		assert.Equal(t, "c2/en\n", h.Out.String())
	})

	t.Run("what identifies each", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "--quiet")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "c1/en\nc1/fa\nc2/en\n", h.Out.String())
	})

	t.Run("as JSON, a row a line", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/files": answer(`{"items":[{"uuid":"f1","name":"cat.png","size":31536000,"owner_uuid":"u1","mimetype":"image/png"}],"pagination":{"total_pages":1,"current_page":1}}`),
		})
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("file"), "tarhche file"), "-A", "--format", "json")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, `{"mimetype":"image/png","name":"cat.png","owner_uuid":"u1","size":31536000,"uuid":"f1"}`+"\n", h.Out.String())
	})

	t.Run("through a template", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "--page", "1", "--format", "{{.title}} ({{.language.name}})")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "Hello (English)\nسلام (Persian)\n", h.Out.String())
	})

	t.Run("a template that cannot be parsed", func(t *testing.T) {
		t.Parallel()

		h := harness(t, articles())

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "--format", "{{")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "parsing the format")
	})

	t.Run("narrowed by the filters given", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/questions": answer(`{"items":[],"pagination":{"total_pages":1,"current_page":1}}`),
		})
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("question"), "tarhche question"), "--kind", "code", "--language-code", "en")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/questions?kind=code&language_code=en&page=1"}, api.routes())
	})

	t.Run("a filter of everybody's is given with --all-users", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/exam-attempts": answer(`{"items":[],"pagination":{"total_pages":1,"current_page":1}}`),
		})
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("exam-attempt"), "tarhche exam-attempt"), "-A", "--user-uuid", "u1", "--exam-uuid", "x1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/exam-attempts?exam_uuid=x1&page=1&user_uuid=u1"}, api.routes())
	})

	t.Run("a filter of everybody's without --all-users", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(nil)
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("exam-attempt"), "tarhche exam-attempt"), "--user-uuid", "u1")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "--user-uuid narrows everybody's exam attempts: add --all-users.")
		assert.Empty(t, api.routes())
	})

	t.Run("a moment as a duration before now, or as it is given", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/audit-logs": answer(`{"items":[]}`),
		})
		h := harness(t, api)

		list := newListCommand(h.Cli, lookup("audit-log"), "tarhche audit-log")
		list.filters.now = func() time.Time { return time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC) }

		status := h.Run(list, "--from", "90m", "--to", "2026-11-22T11:59:00Z")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/audit-logs?from=2026-11-22T10%3A30%3A00Z&to=2026-11-22T11%3A59%3A00Z"}, api.routes())
	})

	t.Run("a page of a listing paged by a cursor, and where the next one starts", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/audit-logs": answer(`{"items":[{"uuid":"g1","action":"articles.index","kind":"read","entity":{"type":"article"},"status":200,"user":{"uuid":"u1","username":"mahdi"},"ip":"10.0.0.1","occurred_at":"2026-11-20T10:00:00Z"}],"next":"cursor-2"}`),
		})
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"), "-A", "--before", "cursor-1", "-q", "--kind", "read")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/audit-logs?before=cursor-1&kind=read"}, api.routes())
		assert.Equal(t, "g1\n", h.Out.String())
		assert.Equal(t, "More are listed with --before cursor-2\n", h.Err.String())
	})

	t.Run("the last page of a listing paged by a cursor", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/audit-logs": answer(`{"items":[]}`),
		})
		h := harness(t, api)

		status := h.Run(newListCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Empty(t, h.Err.String())
	})

	t.Run("a listing of one audience alone, under another name", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{
			"GET /api/dashboard/profile/roles": answer(`{"items":[{"uuid":"r1","translations":{"en":{"title":"Admin"}},"statements":[]}]}`),
		})
		h := harness(t, api)

		list := newListCommand(h.Cli, lookup("role"), "tarhche profile", named("roles", "List the roles one holds"), forAudience(resource.AudienceMine))

		status := h.Run(list)

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/profile/roles?page=1"}, api.routes())
		assert.Equal(t, "UUID      TITLE        DEFAULT\nr1        Admin (en)   \n", h.Out.String())
		assert.Equal(t, "roles", list.Name())
		assert.Equal(t, "List the roles one holds", list.Description())
		assert.Equal(t, "tarhche profile roles [OPTIONS]", list.Usage())
	})

	t.Run("no choice of whose where one audience is served", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newListCommand(h.Cli, lookup("user"), "tarhche user"), "-A")

		assert.Equal(t, console.ExitUsageError, status)
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		list := newListCommand(nil, lookup("license-template"), "tarhche license-template")

		assert.Equal(t, "ls", list.Name())
		assert.Equal(t, "List license templates", list.Description())
		assert.Equal(t, "tarhche license-template ls [OPTIONS]", list.Usage())
	})

	t.Run("arguments are a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newListCommand(h.Cli, lookup("article"), "tarhche article"), "extra")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "accepts no arguments")
	})

	for _, test := range []struct {
		name string
		d    string
		args []string
	}{
		{name: "every page", d: "article"},
		{name: "a page", d: "article", args: []string{"--page", "3"}},
		{name: "a page of a cursor", d: "audit-log"},
	} {
		t.Run("an error reading "+test.name, func(t *testing.T) {
			t.Parallel()

			h := harness(t, newFakeAPI(nil))

			status := h.Run(newListCommand(h.Cli, lookup(test.d), "tarhche "+test.d), test.args...)

			assert.Equal(t, console.ExitFailure, status)
			assert.Contains(t, h.Err.String(), "Error: not found")
		})
	}

	t.Run("items that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := harness(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, _ = rw.Write([]byte(`{"items":[{"uuid":}]}`))
		}))

		status := h.Run(newListCommand(h.Cli, lookup("permission"), "tarhche"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading the answer")
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newListCommand(cli, lookup("article"), "tarhche article")
		})
	})
}
