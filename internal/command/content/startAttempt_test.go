package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestStartAttemptCommand(t *testing.T) {
	t.Parallel()

	t.Run("with the names its license names, writing the attempt's uuid", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/exams/x1/attempts": answered(http.StatusCreated, `{"uuid":"a1"}`)})
		h := harness(t, api)

		status := h.Run(newStartAttemptCommand(h.Cli, "tarhche exam-attempt"), "--first-name", "Sarah", "--last-name", "Hosseini", "x1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"POST /api/exams/x1/attempts"}, api.routes())
		assert.JSONEq(t, `{"first_name":"Sarah","last_name":"Hosseini"}`, api.requests()[0].Body)
		assert.Equal(t, "a1\n", h.Out.String())
	})

	t.Run("nothing of an answer without one", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"POST /api/exams/x1/attempts": answered(http.StatusCreated, `{}`)}))

		status := h.Run(newStartAttemptCommand(h.Cli, "tarhche exam-attempt"), "x1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Empty(t, h.Out.String())
	})

	t.Run("an exam that is not there", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(nil))

		status := h.Run(newStartAttemptCommand(h.Cli, "tarhche exam-attempt"), "x9")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: no such exam: x9\n", h.Err.String())
	})

	for name, args := range map[string][]string{
		"no uuid":          nil,
		"a uuid it is not": {"a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			h := harness(t, nil)

			assert.Equal(t, console.ExitUsageError, h.Run(newStartAttemptCommand(h.Cli, "tarhche exam-attempt"), args...))
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		start := newStartAttemptCommand(nil, "tarhche exam-attempt")

		assert.Equal(t, "start", start.Name())
		assert.Equal(t, "Start an attempt at an exam, or go on with the one in progress", start.Description())
		assert.Equal(t, "tarhche exam-attempt start [OPTIONS] EXAM_UUID", start.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command { return newStartAttemptCommand(cli, "tarhche exam-attempt") }, "x1")
	})
}
