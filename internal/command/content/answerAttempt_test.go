package content

import (
	"net/http"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/command"
)

func TestAnswerAttemptCommand(t *testing.T) {
	t.Parallel()

	const page = `{"stage":0,"page":0,"answers":[{"question_uuid":"q1","option_ids":["o2"]}]}`

	t.Run("a page from a file, writing the attempt's uuid", func(t *testing.T) {
		t.Parallel()

		api := newFakeAPI(map[string][]reply{"POST /api/exam-attempts/a1/answers": answered(http.StatusNoContent, "")})
		h := harness(t, api)

		status := h.Run(newAnswerAttemptCommand(h.Cli, "tarhche exam-attempt"), "-f", fileOf(t, page), "a1")

		require.Equal(t, console.ExitSuccess, status, h.Err.String())
		assert.Equal(t, []string{"POST /api/exam-attempts/a1/answers"}, api.routes())
		assert.JSONEq(t, page, api.requests()[0].Body)
		assert.Equal(t, "a1\n", h.Out.String())
	})

	t.Run("what the api refuses", func(t *testing.T) {
		t.Parallel()

		h := harness(t, newFakeAPI(map[string][]reply{"POST /api/exam-attempts/a1/answers": answered(http.StatusBadRequest, `{"errors":{"page":"invalid_value"}}`)}))

		status := h.Run(newAnswerAttemptCommand(h.Cli, "tarhche exam-attempt"), "-f", fileOf(t, page), "a1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Equal(t, "Error: a1: page: invalid_value\n", h.Err.String())
	})

	t.Run("a file that holds no object", func(t *testing.T) {
		t.Parallel()

		h := harness(t, nil)

		status := h.Run(newAnswerAttemptCommand(h.Cli, "tarhche exam-attempt"), "-f", fileOf(t, `"no"`), "a1")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, h.Err.String(), "holds no JSON object")
	})

	for name, args := range map[string][]string{
		"no file":          {"a1"},
		"no uuid":          {"-f", "x"},
		"a uuid it is not": {"-f", "x", "a/b"},
	} {
		t.Run(name+" is a usage error", func(t *testing.T) {
			t.Parallel()

			h := harness(t, nil)

			assert.Equal(t, console.ExitUsageError, h.Run(newAnswerAttemptCommand(h.Cli, "tarhche exam-attempt"), args...))
		})
	}

	t.Run("its name and what it does", func(t *testing.T) {
		t.Parallel()

		answer := newAnswerAttemptCommand(nil, "tarhche exam-attempt")

		assert.Equal(t, "answer", answer.Name())
		assert.Equal(t, "Answer a page of one's attempt from a JSON file", answer.Description())
		assert.Equal(t, "tarhche exam-attempt answer -f FILE UUID", answer.Usage())
	})

	t.Run("without a client", func(t *testing.T) {
		t.Parallel()

		noClient(t, func(cli *command.Cli) console.Command {
			return newAnswerAttemptCommand(cli, "tarhche exam-attempt")
		}, "-f", fileOf(t, page), "a1")
	})
}
