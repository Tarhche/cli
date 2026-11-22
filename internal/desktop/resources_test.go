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
// both audiences, one's own reading through everybody's routes where it has
// none.
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
	Table:   `{{.correlation_uuid}}\t{{.language_code}}\t{{.title}}\t{{.views}}`,
	Headers: map[string]string{"correlation_uuid": "ID", "language_code": "LANGUAGE"},
}

// messages are a resource keyed by its uuid, with no table of its own, and
// actions of each audience.
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
		{Name: "activity", Method: http.MethodGet, Path: "/api/dashboard/contact-us/activity"},
		{Name: "activity", Method: http.MethodGet, Path: "/api/dashboard/my/contact-us/activity", Mine: true},
	},
}

// bookmarks are one's own alone, deleted with their key in the body.
var bookmarks = resource.Definition{
	Name:    "bookmark",
	Plural:  "bookmarks",
	Title:   "Bookmarks",
	Section: resource.SectionAccount,
	Key:     []string{"object_uuid", "language_code"},
	Mine:    resource.Paths{List: "/api/dashboard/my/bookmarks", Delete: "/api/dashboard/my/bookmarks"},
}

func TestResources(t *testing.T) {
	t.Parallel()

	newResources := func(t *testing.T, api http.Handler, definitions ...resource.Definition) *desktop.Resources {
		t.Helper()

		w, _ := newWorkspace(t, api)

		return desktop.NewResources(w, func() []resource.Definition { return definitions })
	}

	page := `{"items":[{"correlation_uuid":"c-1","language_code":"en","title":"Hello","views":31536000},{"correlation_uuid":"c-2","language_code":"fa"}],"pagination":{"total_pages":3,"current_page":2}}`

	t.Run("the definitions are every resource, as its pages are built", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, articles, messages, bookmarks)

		assert.Equal(t, []desktop.ResourceDefinition{
			{
				Name:        "article",
				Plural:      "articles",
				Title:       "Articles",
				Description: "What is written",
				Section:     resource.SectionContent,
				Key:         []string{"correlation_uuid", "language_code"},
				Audiences:   []string{"mine", "everyone"},
				Mine:        desktop.Routes{List: true, Get: true, Create: true, Update: true, Delete: true},
				Everyone:    desktop.Routes{List: true, Get: true, Create: true, Update: true, Delete: true},
				Actions:     []desktop.ResourceAction{},
				Filters:     []desktop.ResourceFilter{},
				Columns:     []string{"ID", "LANGUAGE", "TITLE", "VIEWS"},
			},
			{
				Name:      "message",
				Plural:    "messages",
				Title:     "Messages",
				Section:   resource.SectionAdministration,
				Key:       []string{"uuid"},
				Audiences: []string{"everyone"},
				Mine:      desktop.Routes{List: true, Update: true},
				Everyone:  desktop.Routes{List: true, Update: true},
				Actions: []desktop.ResourceAction{
					{Name: "read", Description: "Marks it read", Keyed: true},
					{Name: "archive", Mine: true, Keyed: true},
					{Name: "activity"},
					{Name: "activity", Mine: true},
				},
				Filters: []desktop.ResourceFilter{},
				Columns: []string{"UUID"},
			},
			{
				Name:      "bookmark",
				Plural:    "bookmarks",
				Title:     "Bookmarks",
				Section:   resource.SectionAccount,
				Key:       []string{"object_uuid", "language_code"},
				Audiences: []string{"mine"},
				Mine:      desktop.Routes{List: true, Delete: true},
				Actions:   []desktop.ResourceAction{},
				Filters:   []desktop.ResourceFilter{},
				Columns:   []string{"OBJECT UUID", "LANGUAGE CODE"},
			},
		}, resources.Definitions())
	})

	t.Run("no definitions are none", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []desktop.ResourceDefinition{}, newResources(t, nil).Definitions())
	})

	t.Run("a listing is laid out as its table, its numbers as they were written", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, page)
		resources := newResources(t, recorded, articles)

		listing, err := resources.List(context.Background(), "articles", true, desktop.ListQuery{Page: 2, Filters: map[string]string{"language_code": "en", "kind": ""}})

		require.NoError(t, err)
		assert.Equal(t, request{Method: http.MethodGet, Path: "/api/dashboard/articles", Query: "language_code=en&page=2", Token: "Bearer test-token"}, recorded.last(t))
		assert.Equal(t, []string{"ID", "LANGUAGE", "TITLE", "VIEWS"}, listing.Columns)
		assert.Equal(t, uint(3), listing.TotalPages)
		assert.Equal(t, uint(2), listing.CurrentPage)
		require.Len(t, listing.Rows, 2)
		assert.Equal(t, []string{"c-1", "en"}, listing.Rows[0].Key)
		assert.Equal(t, []string{"c-1", "en", "Hello", "31536000"}, listing.Rows[0].Cells)
		assert.JSONEq(t, `{"correlation_uuid":"c-1","language_code":"en","title":"Hello","views":31536000}`, string(listing.Rows[0].Item))
		assert.Equal(t, []string{"c-2", "fa", "", ""}, listing.Rows[1].Cells)
	})

	t.Run("one's own are listed through one's own routes", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{"items":[],"pagination":{"total_pages":0,"current_page":1}}`)
		resources := newResources(t, recorded, articles)

		listing, err := resources.List(context.Background(), "article", false, desktop.ListQuery{})

		require.NoError(t, err)
		assert.Equal(t, "/api/dashboard/my/articles", recorded.last(t).Path)
		assert.Empty(t, recorded.last(t).Query)
		assert.Equal(t, []desktop.Row{}, listing.Rows)
	})

	t.Run("a row that says no key has none", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1","body":"hi"},{"body":"no key"},{"uuid":""}]}`), messages)

		listing, err := resources.List(context.Background(), "messages", true, desktop.ListQuery{Page: 1})

		require.NoError(t, err)
		assert.Equal(t, []string{"UUID"}, listing.Columns)
		assert.Equal(t, []string{"m-1"}, listing.Rows[0].Key)
		assert.Equal(t, []string{"m-1"}, listing.Rows[0].Cells)
		assert.Equal(t, []string{}, listing.Rows[1].Key)
		assert.Equal(t, []string{}, listing.Rows[2].Key)
	})

	t.Run("a table that cannot be read or written is an error", func(t *testing.T) {
		t.Parallel()

		unparsable := messages
		unparsable.Table = `{{.uuid`

		unwritable := messages
		unwritable.Name = "unwritable"
		unwritable.Table = `{{.missing | upper}}`

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1"}]}`), unparsable, unwritable)

		_, parseErr := resources.List(context.Background(), "messages", true, desktop.ListQuery{})
		_, writeErr := resources.List(context.Background(), "unwritable", true, desktop.ListQuery{})

		assert.Error(t, parseErr)
		assert.Error(t, writeErr)
	})

	t.Run("what the API refuses is an error", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusForbidden, `{"code":"forbidden"}`), articles)

		_, listErr := resources.List(context.Background(), "articles", true, desktop.ListQuery{})
		_, getErr := resources.Get(context.Background(), "articles", true, []string{"c-1", "en"})

		assert.True(t, api.IsForbidden(listErr))
		assert.True(t, api.IsForbidden(getErr))
	})

	t.Run("a resource is read by its key, escaped", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{"title":"Hello"}`)
		resources := newResources(t, recorded, articles)

		got, err := resources.Get(context.Background(), "articles", false, []string{"c 1", "en"})

		require.NoError(t, err)
		assert.JSONEq(t, `{"title":"Hello"}`, string(got))
		assert.Equal(t, "/api/dashboard/my/articles/c%201/en", recorded.last(t).Path)
	})

	t.Run("a resource is created from its JSON object, its numbers as they were written", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusCreated, `{"uuid":"c-9"}`)
		resources := newResources(t, recorded, articles)

		got, err := resources.Create(context.Background(), "articles", false, `[{"title":"New","views":31536000}]`)

		require.NoError(t, err)
		assert.JSONEq(t, `{"uuid":"c-9"}`, string(got))
		assert.Equal(t, http.MethodPost, recorded.last(t).Method)
		assert.Equal(t, "/api/dashboard/articles", recorded.last(t).Path)
		assert.Equal(t, `{"title":"New","views":31536000}`, recorded.last(t).Body)
	})

	t.Run("a resource is changed to its JSON, its key in the body where the route does not name it", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusNoContent, ``)
		resources := newResources(t, recorded, articles, messages)

		articleAnswer, articleErr := resources.Update(context.Background(), "articles", true, []string{"c-1", "en"}, `{"correlation_uuid":"changed","title":"Changed"}`)
		article := recorded.last(t)

		_, messageErr := resources.Update(context.Background(), "messages", true, nil, `{"uuid":"m 1","read":true}`)
		message := recorded.last(t)

		require.NoError(t, articleErr)
		require.NoError(t, messageErr)
		assert.Nil(t, articleAnswer)
		assert.Equal(t, http.MethodPut, article.Method)
		assert.Equal(t, "/api/dashboard/articles", article.Path)
		assert.JSONEq(t, `{"correlation_uuid":"c-1","language_code":"en","title":"Changed"}`, article.Body)
		assert.Equal(t, "/api/dashboard/contact-us/m%201", message.Path)
		assert.JSONEq(t, `{"uuid":"m 1","read":true}`, message.Body)
	})

	t.Run("a change that says not which it is is refused before it is sent", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{}`)
		resources := newResources(t, recorded, articles)

		_, err := resources.Update(context.Background(), "articles", true, nil, `{"correlation_uuid":"c-1","title":"Changed"}`)

		var invalid desktop.InvalidError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, map[string]string{"language_code": "say which article it is: its language_code"}, invalid.Fields)
		assert.Zero(t, recorded.count())
	})

	t.Run("what is not JSON, or not an object, is refused before it is sent", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{}`)
		resources := newResources(t, recorded, articles)

		_, createErr := resources.Create(context.Background(), "articles", true, `{"title":`)
		_, updateErr := resources.Update(context.Background(), "articles", true, []string{"c-1", "en"}, `{"title":`)
		_, arrayErr := resources.Update(context.Background(), "articles", true, []string{"c-1", "en"}, `[1, 2]`)

		var invalid desktop.InvalidError
		require.ErrorAs(t, createErr, &invalid)
		assert.Equal(t, map[string]string{"json": "it is not valid JSON"}, invalid.Fields)
		require.ErrorAs(t, updateErr, &invalid)
		require.ErrorAs(t, arrayErr, &invalid)
		assert.Equal(t, map[string]string{"json": "it has to be an object"}, invalid.Fields)
		assert.Zero(t, recorded.count())
	})

	t.Run("a resource is deleted by its key, in its path or its body", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusNoContent, ``)
		resources := newResources(t, recorded, articles, bookmarks)

		articleErr := resources.Delete(context.Background(), "articles", true, []string{"c-1", "fa"})
		article := recorded.last(t)

		bookmarkErr := resources.Delete(context.Background(), "bookmarks", false, []string{"c-1", "fa"})
		bookmark := recorded.last(t)

		require.NoError(t, articleErr)
		require.NoError(t, bookmarkErr)
		assert.Equal(t, request{Method: http.MethodDelete, Path: "/api/dashboard/articles/c-1/fa", Token: "Bearer test-token"}, article)
		assert.Equal(t, http.MethodDelete, bookmark.Method)
		assert.Equal(t, "/api/dashboard/my/bookmarks", bookmark.Path)
		assert.JSONEq(t, `{"object_uuid":"c-1","language_code":"fa"}`, bookmark.Body)
	})

	t.Run("what a resource does not serve is refused before it is sent", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{}`)
		resources := newResources(t, recorded, messages, bookmarks)

		_, listErr := resources.List(context.Background(), "bookmarks", true, desktop.ListQuery{})
		_, getErr := resources.Get(context.Background(), "messages", true, []string{"m-1"})
		_, createErr := resources.Create(context.Background(), "messages", true, `{}`)
		_, updateErr := resources.Update(context.Background(), "bookmarks", false, []string{"b-1", "en"}, `{}`)
		deleteErr := resources.Delete(context.Background(), "messages", true, []string{"m-1"})

		assert.EqualError(t, listErr, "Bookmarks cannot be listed here")
		assert.EqualError(t, getErr, "Messages cannot be read here")
		assert.EqualError(t, createErr, "Messages cannot be created here")
		assert.EqualError(t, updateErr, "Bookmarks cannot be changed here")
		assert.EqualError(t, deleteErr, "Messages cannot be deleted here")
		assert.Zero(t, recorded.count())
	})

	t.Run("a resource is asked an action of one thing, by its key", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{"read":true}`)
		resources := newResources(t, recorded, messages)

		read, readErr := resources.Action(context.Background(), "messages", "read", true, []string{"m-1"})
		readRequest := recorded.last(t)

		_, archiveErr := resources.Action(context.Background(), "message", "archive", true, []string{"m-1"})
		archiveRequest := recorded.last(t)

		require.NoError(t, readErr)
		require.NoError(t, archiveErr)
		assert.JSONEq(t, `{"read":true}`, string(read))
		assert.Equal(t, http.MethodPut, readRequest.Method)
		assert.Equal(t, "/api/dashboard/contact-us/m-1/read", readRequest.Path)
		assert.Empty(t, readRequest.Body)
		assert.Equal(t, http.MethodPost, archiveRequest.Method)
		assert.Equal(t, "/api/dashboard/my/contact-us/m-1/archive", archiveRequest.Path)
	})

	t.Run("an action of the resource is asked of the audience asked", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{"points":[]}`)
		resources := newResources(t, recorded, messages)

		everybody, everybodyErr := resources.Action(context.Background(), "messages", "activity", true, nil)
		everybodyPath := recorded.last(t).Path

		_, mineErr := resources.Action(context.Background(), "messages", "activity", false, nil)
		mine := recorded.last(t)

		require.NoError(t, everybodyErr)
		require.NoError(t, mineErr)
		assert.JSONEq(t, `{"points":[]}`, string(everybody))
		assert.Equal(t, "/api/dashboard/contact-us/activity", everybodyPath)
		assert.Equal(t, http.MethodGet, mine.Method)
		assert.Equal(t, "/api/dashboard/my/contact-us/activity", mine.Path)
	})

	t.Run("an action of one thing needs its key", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, messages)

		_, err := resources.Action(context.Background(), "messages", "read", true, nil)

		assert.EqualError(t, err, "say which message to read")
	})

	t.Run("an action a resource does not have is refused", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, messages)

		_, err := resources.Action(context.Background(), "messages", "shred", true, []string{"m-1"})

		assert.EqualError(t, err, "Messages cannot be asked to shred")
	})

	t.Run("a resource that is not described is refused", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, articles)

		_, listErr := resources.List(context.Background(), "spells", true, desktop.ListQuery{})
		_, getErr := resources.Get(context.Background(), "spells", true, nil)
		_, createErr := resources.Create(context.Background(), "spells", true, `{}`)
		_, updateErr := resources.Update(context.Background(), "spells", true, nil, `{}`)
		deleteErr := resources.Delete(context.Background(), "spells", true, nil)
		_, actionErr := resources.Action(context.Background(), "spells", "cast", true, nil)

		for _, err := range []error{listErr, getErr, createErr, updateErr, deleteErr, actionErr} {
			assert.EqualError(t, err, `there is no resource called "spells"`)
		}
	})

	t.Run("a context whose configuration cannot be read is not asked", func(t *testing.T) {
		t.Parallel()

		resources := desktop.NewResources(brokenWorkspace(t), func() []resource.Definition { return []resource.Definition{articles} })

		_, listErr := resources.List(context.Background(), "articles", true, desktop.ListQuery{})
		_, getErr := resources.Get(context.Background(), "articles", true, []string{"c-1", "en"})

		assert.Error(t, listErr)
		assert.Error(t, getErr)
	})

	t.Run("an empty answer is none", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, "  \n"), articles)

		got, err := resources.Get(context.Background(), "articles", true, []string{"c-1", "en"})

		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, "null", jsonOf(t, got))
	})
}
