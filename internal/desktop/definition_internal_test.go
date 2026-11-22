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

	"github.com/Tarhche/cli/internal/access"
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

	t.Run("one's own route unless everybody's are asked for, and everybody's where one's own is not served", func(t *testing.T) {
		t.Parallel()

		d := resource.Definition{Mine: resource.Paths{List: "/my"}, Everyone: resource.Paths{List: "/all", Delete: "/all/{uuid}"}}

		assert.Equal(t, "/my", routeOf(d, operationList, false))
		assert.Equal(t, "/all", routeOf(d, operationList, true))
		assert.Equal(t, "/all/{uuid}", routeOf(d, operationDelete, false))
		assert.Empty(t, routeOf(d, operationCreate, true))
	})

	t.Run("the audiences offered are those with a listing the session may ask", func(t *testing.T) {
		t.Parallel()

		d := resource.Definition{Mine: resource.Paths{Get: "/my/{uuid}"}, Everyone: resource.Paths{List: "/all"}}
		users := resource.Definition{Mine: resource.Paths{List: "/api/dashboard/profile/roles"}, Everyone: resource.Paths{List: "/api/dashboard/users"}}

		assert.Equal(t, []string{"everyone"}, audiencesOf(d, nil))
		assert.Equal(t, []string{}, audiencesOf(resource.Definition{}, nil))
		assert.Equal(t, []string{"mine"}, audiencesOf(users, nil))
		assert.Equal(t, []string{"mine", "everyone"}, audiencesOf(users, access.Granted{"users.index"}))
	})

	t.Run("an action is asked with its method, and POST when it names none", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.MethodPut, methodOf(resource.Action{Method: http.MethodPut}))
		assert.Equal(t, http.MethodPost, methodOf(resource.Action{}))
	})

	t.Run("filters are offered with the listings that take them", func(t *testing.T) {
		t.Parallel()

		d := resource.Definition{Filters: []resource.Filter{
			{Name: "kind", Description: "of a kind", Values: []string{"read"}, Audiences: []resource.Audience{resource.AudienceEveryone}},
			{Name: "from", Moment: true},
		}}

		assert.Equal(t, []ResourceFilter{
			{Name: "kind", Description: "of a kind", Values: []string{"read"}, Audiences: []string{"everyone"}},
			{Name: "from", Values: []string{}, Moment: true, Audiences: []string{}},
		}, filtersOf(d))
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

}
