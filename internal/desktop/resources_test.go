package desktop_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/desktop"
	"github.com/Tarhche/cli/internal/resource"
)

// articles are a resource keyed by two fields, with a table of its own and
// both audiences.
var articles = resource.Definition{
	Name:        "article",
	Plural:      "articles",
	Title:       "Articles",
	Description: "What is written",
	Section:     resource.SectionContent,
	Key:         []string{"correlation_uuid", "language_code"},
	Everyone: resource.Paths{
		List:   "/api/dashboard/articles",
		Get:    "/api/dashboard/articles/{correlation_uuid}/{language_code}",
		Create: "/api/dashboard/articles",
		Update: "/api/dashboard/articles",
		Delete: "/api/dashboard/articles/{correlation_uuid}/{language_code}",
	},
	Mine: resource.Paths{
		List: "/api/dashboard/my/articles",
		Get:  "/api/dashboard/my/articles/{correlation_uuid}/{language_code}",
	},
	Table:   `{{.correlation_uuid}}\t{{.language_code}}\t{{.title}}`,
	Headers: map[string]string{"correlation_uuid": "ID", "language_code": "LANGUAGE"},
}

// messages are a resource keyed by its uuid, with no table of its own and
// actions.
var messages = resource.Definition{
	Name:     "message",
	Plural:   "messages",
	Title:    "Messages",
	Section:  resource.SectionAdministration,
	Key:      []string{"uuid"},
	Everyone: resource.Paths{List: "/api/dashboard/contact-us", Update: "/api/dashboard/contact-us/{uuid}"},
	Actions: []resource.Action{
		{Name: "read", Description: "Marks it read", Method: http.MethodPut, Path: "/api/dashboard/contact-us/{uuid}/read"},
		{Name: "archive", Path: "/api/dashboard/my/contact-us/{uuid}/archive", Mine: true},
	},
}

func TestResources(t *testing.T) {
	t.Parallel()

	newResources := func(t *testing.T, api http.Handler, definitions ...resource.Definition) *desktop.Resources {
		t.Helper()

		w, _ := newWorkspace(t, api)

		return desktop.NewResources(w, func() []resource.Definition { return definitions })
	}

	page := `{"items":[{"correlation_uuid":"c-1","language_code":"en","title":"Hello"},{"correlation_uuid":"c-2","language_code":"fa"}],"pagination":{"total_pages":3,"current_page":2}}`

	t.Run("the definitions are every resource, as its pages are built", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, articles, messages)

		assert.Equal(t, []desktop.ResourceDefinition{
			{
				Name:        "article",
				Plural:      "articles",
				Title:       "Articles",
				Description: "What is written",
				Section:     resource.SectionContent,
				Key:         []string{"correlation_uuid", "language_code"},
				Everyone:    desktop.Routes{List: true, Get: true, Create: true, Update: true, Delete: true},
				Mine:        desktop.Routes{List: true, Get: true},
				Actions:     []desktop.ResourceAction{},
				Columns:     []string{"ID", "LANGUAGE", "TITLE"},
			},
			{
				Name:     "message",
				Plural:   "messages",
				Title:    "Messages",
				Section:  resource.SectionAdministration,
				Key:      []string{"uuid"},
				Everyone: desktop.Routes{List: true, Update: true},
				Actions: []desktop.ResourceAction{
					{Name: "read", Description: "Marks it read"},
					{Name: "archive", Mine: true},
				},
				Columns: []string{"UUID"},
			},
		}, resources.Definitions())
	})

	t.Run("no definitions are none", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []desktop.ResourceDefinition{}, newResources(t, nil).Definitions())
	})

	t.Run("a listing is laid out as its table", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, page)
		resources := newResources(t, api, articles)

		listing, err := resources.List(context.Background(), "articles", false, 2)

		require.NoError(t, err)
		assert.Equal(t, request{Method: http.MethodGet, Path: "/api/dashboard/articles", Query: "page=2", Token: "Bearer test-token"}, api.last(t))
		assert.Equal(t, []string{"ID", "LANGUAGE", "TITLE"}, listing.Columns)
		assert.Equal(t, uint(3), listing.TotalPages)
		assert.Equal(t, uint(2), listing.CurrentPage)
		require.Len(t, listing.Rows, 2)
		assert.Equal(t, []string{"c-1", "en"}, listing.Rows[0].Key)
		assert.Equal(t, []string{"c-1", "en", "Hello"}, listing.Rows[0].Cells)
		assert.JSONEq(t, `{"correlation_uuid":"c-1","language_code":"en","title":"Hello"}`, string(listing.Rows[0].Item))
		assert.Equal(t, []string{"c-2", "fa", ""}, listing.Rows[1].Cells)
	})

	t.Run("one's own are listed through one's own routes", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{"items":[],"pagination":{"total_pages":0,"current_page":1}}`)
		resources := newResources(t, api, articles)

		listing, err := resources.List(context.Background(), "article", true, 0)

		require.NoError(t, err)
		assert.Equal(t, "/api/dashboard/my/articles", api.last(t).Path)
		assert.Empty(t, api.last(t).Query)
		assert.Equal(t, []desktop.Row{}, listing.Rows)
	})

	t.Run("a resource with no table of its own is listed by its key", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1","body":"hi"},{"body":"no key"}]}`), messages)

		listing, err := resources.List(context.Background(), "messages", false, 1)

		require.NoError(t, err)
		assert.Equal(t, []string{"UUID"}, listing.Columns)
		assert.Equal(t, []string{"m-1"}, listing.Rows[0].Cells)
		assert.Equal(t, []string{""}, listing.Rows[1].Key)
	})

	t.Run("a table that cannot be read or written is an error", func(t *testing.T) {
		t.Parallel()

		unparsable := messages
		unparsable.Table = `{{.uuid`

		unwritable := messages
		unwritable.Name = "unwritable"
		unwritable.Table = `{{.missing | upper}}`

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1"}]}`), unparsable, unwritable)

		_, parseErr := resources.List(context.Background(), "messages", false, 1)
		_, writeErr := resources.List(context.Background(), "unwritable", false, 1)

		assert.Error(t, parseErr)
		assert.Error(t, writeErr)
	})

	t.Run("what the API refuses is an error", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusForbidden, `{"code":"forbidden"}`), articles)

		_, listErr := resources.List(context.Background(), "articles", false, 1)
		_, getErr := resources.Get(context.Background(), "articles", false, []string{"c-1", "en"})

		assert.True(t, api.IsForbidden(listErr))
		assert.True(t, api.IsForbidden(getErr))
	})

	t.Run("a resource is read by its key, escaped", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{"title":"Hello"}`)
		resources := newResources(t, api, articles)

		got, err := resources.Get(context.Background(), "articles", true, []string{"c 1", "en"})

		require.NoError(t, err)
		assert.JSONEq(t, `{"title":"Hello"}`, string(got))
		assert.Equal(t, "/api/dashboard/my/articles/c%201/en", api.last(t).Path)
	})

	t.Run("a resource is created from its JSON", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusCreated, `{"uuid":"c-9"}`)
		resources := newResources(t, api, articles)

		got, err := resources.Create(context.Background(), "articles", false, `{"title":"New"}`)

		require.NoError(t, err)
		assert.JSONEq(t, `{"uuid":"c-9"}`, string(got))
		assert.Equal(t, request{Method: http.MethodPost, Path: "/api/dashboard/articles", Body: `{"title":"New"}`, Token: "Bearer test-token"}, api.last(t))
	})

	t.Run("a resource is changed to its JSON, which carries its key", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusNoContent, ``)
		resources := newResources(t, api, articles, messages)

		articleAnswer, articleErr := resources.Update(context.Background(), "articles", false, `{"correlation_uuid":"c-1","language_code":"en","title":"Changed"}`)
		articlePath := api.last(t).Path

		_, messageErr := resources.Update(context.Background(), "messages", false, `{"uuid":"m 1","read":true}`)
		message := api.last(t)

		require.NoError(t, articleErr)
		require.NoError(t, messageErr)
		assert.Nil(t, articleAnswer)
		assert.Equal(t, "/api/dashboard/articles", articlePath)
		assert.Equal(t, http.MethodPut, message.Method)
		assert.Equal(t, "/api/dashboard/contact-us/m%201", message.Path)
		assert.JSONEq(t, `{"uuid":"m 1","read":true}`, message.Body)
	})

	t.Run("what is not JSON, or not an object, is refused before it is sent", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{}`)
		resources := newResources(t, api, articles)

		_, createErr := resources.Create(context.Background(), "articles", false, `{"title":`)
		_, updateErr := resources.Update(context.Background(), "articles", false, `{"title":`)
		_, arrayErr := resources.Update(context.Background(), "articles", false, `[1, 2]`)

		var invalid desktop.InvalidError
		require.ErrorAs(t, createErr, &invalid)
		assert.Equal(t, map[string]string{"json": "it is not valid JSON"}, invalid.Fields)
		require.ErrorAs(t, updateErr, &invalid)
		require.ErrorAs(t, arrayErr, &invalid)
		assert.Equal(t, map[string]string{"json": "it has to be an object"}, invalid.Fields)
		assert.Zero(t, api.count())
	})

	t.Run("a resource is deleted by its key", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusNoContent, ``)
		resources := newResources(t, api, articles)

		err := resources.Delete(context.Background(), "articles", false, []string{"c-1", "fa"})

		require.NoError(t, err)
		assert.Equal(t, request{Method: http.MethodDelete, Path: "/api/dashboard/articles/c-1/fa", Token: "Bearer test-token"}, api.last(t))
	})

	t.Run("what a resource's audience does not serve is refused before it is sent", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{}`)
		resources := newResources(t, api, articles, messages)

		_, listErr := resources.List(context.Background(), "messages", true, 1)
		_, getErr := resources.Get(context.Background(), "messages", false, []string{"m-1"})
		_, createErr := resources.Create(context.Background(), "articles", true, `{}`)
		_, updateErr := resources.Update(context.Background(), "articles", true, `{}`)
		deleteErr := resources.Delete(context.Background(), "articles", true, []string{"c-1", "en"})

		assert.EqualError(t, listErr, "Messages of one's own cannot be listed here")
		assert.EqualError(t, getErr, "Messages of anybody's cannot be read here")
		assert.EqualError(t, createErr, "Articles of one's own cannot be created here")
		assert.EqualError(t, updateErr, "Articles of one's own cannot be changed here")
		assert.EqualError(t, deleteErr, "Articles of one's own cannot be deleted here")
		assert.Zero(t, api.count())
	})

	t.Run("a resource is asked an action by its key", func(t *testing.T) {
		t.Parallel()

		api := newRecorder(http.StatusOK, `{"read":true}`)
		resources := newResources(t, api, messages)

		read, readErr := resources.Action(context.Background(), "messages", "read", []string{"m-1"})
		readRequest := api.last(t)

		_, archiveErr := resources.Action(context.Background(), "message", "archive", []string{"m-1"})
		archiveRequest := api.last(t)

		require.NoError(t, readErr)
		require.NoError(t, archiveErr)
		assert.JSONEq(t, `{"read":true}`, string(read))
		assert.Equal(t, http.MethodPut, readRequest.Method)
		assert.Equal(t, "/api/dashboard/contact-us/m-1/read", readRequest.Path)
		assert.Equal(t, http.MethodPost, archiveRequest.Method)
		assert.Equal(t, "/api/dashboard/my/contact-us/m-1/archive", archiveRequest.Path)
	})

	t.Run("an action a resource does not have is refused", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, messages)

		_, err := resources.Action(context.Background(), "messages", "shred", []string{"m-1"})

		assert.EqualError(t, err, "Messages cannot be asked to shred")
	})

	t.Run("a resource that is not described is refused", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, articles)

		_, listErr := resources.List(context.Background(), "spells", false, 1)
		_, getErr := resources.Get(context.Background(), "spells", false, nil)
		_, createErr := resources.Create(context.Background(), "spells", false, `{}`)
		_, updateErr := resources.Update(context.Background(), "spells", false, `{}`)
		deleteErr := resources.Delete(context.Background(), "spells", false, nil)
		_, actionErr := resources.Action(context.Background(), "spells", "cast", nil)

		for _, err := range []error{listErr, getErr, createErr, updateErr, deleteErr, actionErr} {
			assert.EqualError(t, err, `there is no resource called "spells"`)
		}
	})

	t.Run("a context whose configuration cannot be read is not asked", func(t *testing.T) {
		t.Parallel()

		resources := desktop.NewResources(brokenWorkspace(t), func() []resource.Definition { return []resource.Definition{articles} })

		_, listErr := resources.List(context.Background(), "articles", false, 1)
		_, getErr := resources.Get(context.Background(), "articles", false, []string{"c-1", "en"})

		assert.Error(t, listErr)
		assert.Error(t, getErr)
	})

	t.Run("an empty answer is none", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, "  \n"), articles)

		got, err := resources.Get(context.Background(), "articles", false, []string{"c-1", "en"})

		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, "null", jsonOf(t, got))
	})
}
