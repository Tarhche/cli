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

func TestActivityCommand(t *testing.T) {
	t.Parallel()

	const activity = `{"from":"2026-11-22T11:00:00Z","to":"2026-11-22T12:00:00Z","step":1800,"actions":["articles.index","auth.login"],
		"points":[{"at":"2026-11-22T11:00:00Z","total":12,"actions":{"articles.index":10,"auth.login":1},"other":1},
			{"at":"2026-11-22T11:30:00Z","total":0,"other":0}]}`

	logs := func() *fakeAPI {
		return newFakeAPI(map[string][]reply{
			"GET /api/dashboard/my/audit-logs/activity": answer(activity),
			"GET /api/dashboard/audit-logs/activity":    answer(`{"actions":[],"points":[]}`),
		})
	}

	t.Run("one's own by default, a step a row, an action taken most a column", func(t *testing.T) {
		t.Parallel()

		api := logs()
		h := harness(t, api)

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"))

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/my/audit-logs/activity"}, api.routes())
		assert.Equal(t, ""+
			"AT                     TOTAL     ARTICLES.INDEX   AUTH.LOGIN   OTHER\n"+
			"2026-11-22T11:00:00Z   12        10               1            1\n"+
			"2026-11-22T11:30:00Z   0         0                0            0\n", h.Out.String())
	})

	t.Run("everybody's when asked, narrowed by the filters given", func(t *testing.T) {
		t.Parallel()

		api := logs()
		h := harness(t, api)

		activity := newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log")
		activity.filters.now = func() time.Time { return time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC) }

		status := h.Run(activity, "-A", "--user-uuid", "u1", "--kind", "read", "--from", "24h")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/audit-logs/activity?from=2026-11-21T12%3A00%3A00Z&kind=read&user_uuid=u1"}, api.routes())
		assert.Equal(t, "AT        TOTAL     OTHER\n", h.Out.String())
	})

	t.Run("the answer as JSON on a line", func(t *testing.T) {
		t.Parallel()

		h := harness(t, logs())

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"), "--format", "json")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Contains(t, h.Out.String(), `{"from":"2026-11-22T11:00:00Z","to":"2026-11-22T12:00:00Z","step":1800,`)
	})

	t.Run("the steps through a template", func(t *testing.T) {
		t.Parallel()

		h := harness(t, logs())

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"), "--format", `{{.at}} {{.total}} {{index .actions "articles.index"}}`)

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, "2026-11-22T11:00:00Z 12 10\n2026-11-22T11:30:00Z 0 <no value>\n", h.Out.String())
	})

	t.Run("a filter of everybody's without --all-users", func(t *testing.T) {
		t.Parallel()

		api := logs()
		h := harness(t, api)

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"), "--user-uuid", "u1")

		assert.Equal(t, console.ExitUsageError, status)
		assert.Contains(t, h.Err.String(), "--user-uuid narrows everybody's audit logs: add --all-users.")
		assert.Empty(t, api.routes())
	})

	t.Run("an answer that cannot be read", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"GET /api/dashboard/my/audit-logs/activity": answer(`{"points":"many"}`)}))

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "reading the answer")
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(nil))

		status := h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"))

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: not found\n", h.Err.String())
	})

	t.Run("everybody's where one's own are not counted", func(t *testing.T) {
		t.Parallel()

		d := resource.Definition{
			Name:    "audit-log",
			Title:   "Audit logs",
			Key:     []string{"uuid"},
			Actions: []resource.Action{{Name: "activity", Description: "Count", Method: http.MethodGet, Path: "/api/dashboard/audit-logs/activity"}},
		}
		api := logs()
		h := harness(t, api)

		require.Equal(t, console.ExitSuccess, h.Run(newActivityCommand(h.Cli, d, "tarhche audit-log")), h.Err.String())
		assert.Equal(t, []string{"GET /api/dashboard/audit-logs/activity"}, api.routes())
		assert.Equal(t, console.ExitUsageError, h.Run(newActivityCommand(h.Cli, d, "tarhche audit-log"), "-A"))
	})

	t.Run("arguments are a usage error", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		assert.Equal(t, console.ExitUsageError, h.Run(newActivityCommand(h.Cli, lookup("audit-log"), "tarhche audit-log"), "x"))
	})

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		activity := newActivityCommand(nil, lookup("audit-log"), "tarhche audit-log")

		assert.Equal(t, "activity", activity.Name())
		assert.Contains(t, activity.Description(), "Count the actions taken a step at a time")
		assert.Equal(t, "tarhche audit-log activity [OPTIONS]", activity.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newActivityCommand(cli, lookup("audit-log"), "tarhche audit-log")
		})
	})
}
