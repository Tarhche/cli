package desktop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/resource"
)

// attempts are a resource whose one action is one's own alone.
var attempts = resource.Definition{
	Name: "attempt",
	Key:  []string{"uuid"},
	Actions: []resource.Action{
		{Name: "finish", Method: http.MethodPost, Path: "/api/exam-attempts/{uuid}/finish", Mine: true},
	},
}

func TestRead(t *testing.T) {
	t.Parallel()

	serve := func(t *testing.T, answer string) (*api.Client, *string) {
		t.Helper()

		var asked string

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			asked = r.URL.RawQuery
			_, _ = io.WriteString(rw, answer)
		}))
		t.Cleanup(server.Close)

		client, err := api.New(server.URL)
		require.NoError(t, err)

		return client, &asked
	}

	t.Run("a listing paged by a cursor is read from where its last page said", func(t *testing.T) {
		t.Parallel()

		client, asked := serve(t, `{"items":[{"uuid":"a-1"}],"next":"n-2"}`)

		listing, raws, err := read(context.Background(), client, "/api/dashboard/audit-logs", "before", ListQuery{Cursor: "n-1", Page: 9, Filters: map[string]string{"kind": "read"}})

		require.NoError(t, err)
		assert.Equal(t, "before=n-1&kind=read", *asked)
		assert.Equal(t, Listing{Next: "n-2"}, listing)
		assert.Equal(t, []json.RawMessage{json.RawMessage(`{"uuid":"a-1"}`)}, raws)
	})

	t.Run("its first page is read from nowhere", func(t *testing.T) {
		t.Parallel()

		client, asked := serve(t, `{"items":[]}`)

		listing, _, err := read(context.Background(), client, "/api/dashboard/audit-logs", "before", ListQuery{})

		require.NoError(t, err)
		assert.Empty(t, *asked)
		assert.Equal(t, Listing{}, listing)
	})

	t.Run("a listing that cannot be read is an error", func(t *testing.T) {
		t.Parallel()

		client, _ := serve(t, `{`)

		_, _, cursorErr := read(context.Background(), client, "/", "before", ListQuery{})
		_, _, pageErr := read(context.Background(), client, "/", "", ListQuery{})

		assert.Error(t, cursorErr)
		assert.Error(t, pageErr)
	})
}

func TestDefinitionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("every operation is served at its route", func(t *testing.T) {
		t.Parallel()

		paths := resource.Paths{List: "/l", Get: "/g", Create: "/c", Update: "/u", Delete: "/d"}

		assert.Equal(t, "/l", pathOf(paths, operationList))
		assert.Equal(t, "/g", pathOf(paths, operationGet))
		assert.Equal(t, "/c", pathOf(paths, operationCreate))
		assert.Equal(t, "/u", pathOf(paths, operationUpdate))
		assert.Equal(t, "/d", pathOf(paths, operationDelete))
	})

	t.Run("an action asked of the other audience is its only one", func(t *testing.T) {
		t.Parallel()

		finish, found := chosenAction(attempts, "finish", true)

		assert.True(t, found)
		assert.Equal(t, attempts.Actions[0], finish)
	})

	t.Run("defaults fill what a body does not say", func(t *testing.T) {
		t.Parallel()

		body := withDefaults(map[string]any{"object_type": "comment", "body": "hi"}, map[string]any{"object_type": "article", "language_code": "en"})

		assert.Equal(t, map[string]any{"object_type": "comment", "body": "hi", "language_code": "en"}, body)
	})

	t.Run("a body of nothing is none", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, requestBody(nil))
		assert.Equal(t, map[string]any{"read": true}, requestBody(map[string]any{"read": true}))
	})

	t.Run("fields are cloned, none as an empty map", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{"read": true}
		clone := cloned(fields)
		clone["read"] = false

		assert.Equal(t, map[string]any{}, cloned(nil))
		assert.Equal(t, map[string]any{"read": true}, fields)
	})

	t.Run("JSON of nothing is null, and what is not JSON is an error", func(t *testing.T) {
		t.Parallel()

		nothing, nothingErr := decodeJSON([]byte(" "))
		_, brokenErr := decodeJSON([]byte("{"))

		assert.NoError(t, nothingErr)
		assert.Nil(t, nothing)
		assert.Error(t, brokenErr)
	})

	t.Run("what the definitions do not say yet is nothing", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []any{"a"}, rowsOf(attempts, []any{"a"}))
		assert.Nil(t, defaultsOf(attempts))
		assert.Nil(t, actionBodyOf(attempts.Actions[0]))
		assert.Empty(t, filtersOf(attempts))
		assert.Empty(t, cursorOf(attempts))
	})
}
