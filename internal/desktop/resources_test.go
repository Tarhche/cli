package desktop_test

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/auth"
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
	App: resource.App{
		New: resource.New{Name: "New article", Path: "/dashboard/articles/new"},
		Links: []resource.Link{
			{Name: "View article", Kind: resource.LinkView, Site: resource.SiteWeb, Path: `{{if .published}}/{{.language_code}}/articles/{{.correlation_uuid}}{{end}}`},
			{Name: "Edit article", Kind: resource.LinkEdit, Site: resource.SiteWeb, Path: `/dashboard/articles/{{.correlation_uuid}}/{{.language_code}}`, Requires: []resource.Operation{resource.OperationGet, resource.OperationUpdate}},
			{Name: "Cover", Kind: resource.LinkView, Site: resource.SiteAPI, Path: `/files/{{.cover}}`, Audiences: []resource.Audience{resource.AudienceEveryone}},
		},
	},
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
		{Name: "read", Description: "Marks it read", Method: http.MethodPut, Path: "/api/dashboard/contact-us/{uuid}/read", Title: "Mark as read", When: `{{not .read}}`},
		{Name: "archive", Path: "/api/dashboard/my/contact-us/{uuid}/archive", Mine: true, Title: "Archive", Confirm: "Archive it?"},
		{Name: "activity", Method: http.MethodGet, Path: "/api/dashboard/contact-us/activity"},
		{Name: "activity", Method: http.MethodGet, Path: "/api/dashboard/my/contact-us/activity", Mine: true, Title: "Activity"},
		{Name: "shred", Path: "/api/dashboard/contact-us/{uuid}/shred"},
	},
	App: resource.App{Protected: `{{.pinned}}`},
}

// exams are a resource whose rows lead to a listing of another, filtered.
var exams = resource.Definition{
	Name:     "exam",
	Plural:   "exams",
	Title:    "Exams",
	Section:  resource.SectionLearning,
	Key:      []string{"uuid"},
	Everyone: resource.Paths{List: "/api/dashboard/exams"},
	App: resource.App{
		Links: []resource.Link{{Name: "Attempts", Kind: resource.LinkList, Site: resource.SiteApp, Resource: "exam-attempt", Filter: "exam_uuid", Path: `{{.uuid}}`}},
	},
}

// examAttempts are what exams' rows lead to.
var examAttempts = resource.Definition{
	Name:     "exam-attempt",
	Plural:   "exam-attempts",
	Title:    "Exam attempts",
	Section:  resource.SectionLearning,
	Key:      []string{"uuid"},
	Everyone: resource.Paths{List: "/api/dashboard/exam-attempts"},
	Mine:     resource.Paths{List: "/api/dashboard/my/exam-attempts"},
}

// hidden is a resource the app has no page of.
var hidden = resource.Definition{
	Name:     "permission",
	Plural:   "permissions",
	Key:      []string{"value"},
	Everyone: resource.Paths{List: "/api/dashboard/permissions"},
	App:      resource.App{Hidden: true},
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

	t.Run("the definitions are every resource the session may list, as its pages are built", func(t *testing.T) {
		t.Parallel()

		w := withWebsite(t, newWorkspaceGranting(t, nil,
			"articles.index", "articles.show", "articles.create", "articles.update", "articles.delete", "self.articles.index", "self.articles.show",
			"contactus.index", "contactus.markAsRead", "permissions.index",
			"self.bookmarks.index", "self.bookmarks.delete",
		), "https://web.example/")

		definitions, err := desktop.NewResources(w, func() []resource.Definition { return []resource.Definition{articles, messages, hidden, bookmarks} }).Definitions(context.Background())

		require.NoError(t, err)
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
				Columns:     []string{"Id", "Language", "Title", "Views"},
				New:         desktop.ResourceNew{Name: "New article", URL: "https://web.example/dashboard/articles/new"},
				Links: []desktop.ResourceLink{
					{Name: "View article", Kind: "view", Audiences: []string{"mine", "everyone"}},
					{Name: "Edit article", Kind: "edit", Audiences: []string{"mine", "everyone"}},
					{Name: "Cover", Kind: "view", Audiences: []string{"everyone"}},
				},
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
					{Name: "read", Description: "Marks it read", Title: "Mark as read", Keyed: true, Allowed: true},
					{Name: "archive", Title: "Archive", Confirm: "Archive it?", Mine: true, Keyed: true, Allowed: true},
					{Name: "activity", Title: "Activity", Mine: true, Allowed: true},
				},
				Filters: []desktop.ResourceFilter{},
				Columns: []string{"Uuid"},
				Links:   []desktop.ResourceLink{},
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
				Columns:   []string{"Object uuid", "Language code"},
				Links:     []desktop.ResourceLink{},
			},
		}, definitions)
	})

	t.Run("a link is offered where the session may open its page, and the context reaches it", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceGranting(t, nil, "self.articles.index", "articles.index", "articles.show")

		definitions, err := desktop.NewResources(w, func() []resource.Definition { return []resource.Definition{articles} }).Definitions(context.Background())

		require.NoError(t, err)
		require.Len(t, definitions, 1)
		assert.Equal(t, desktop.ResourceNew{}, definitions[0].New, "nothing is made where the session may not, nor without a website")
		assert.Equal(t, []desktop.ResourceLink{
			{Name: "View article", Kind: "view", Audiences: []string{}},
			{Name: "Edit article", Kind: "edit", Audiences: []string{}},
			{Name: "Cover", Kind: "view", Audiences: []string{"everyone"}},
		}, definitions[0].Links)
	})

	t.Run("a link to another page of the app is offered where its listing of everybody's is", func(t *testing.T) {
		t.Parallel()

		offered := func(t *testing.T, permissions ...string) []string {
			t.Helper()

			w := newWorkspaceGranting(t, nil, permissions...)

			definitions, err := desktop.NewResources(w, func() []resource.Definition { return []resource.Definition{exams, examAttempts} }).Definitions(context.Background())
			require.NoError(t, err)

			return definitions[0].Links[0].Audiences
		}

		assert.Equal(t, []string{"everyone"}, offered(t, "exams.index", "exam.attempts.index"))
		assert.Equal(t, []string{}, offered(t, "exams.index", "self.exam.attempts.index"))
		assert.Equal(t, []string{}, offered(t, "exams.index"))
	})

	t.Run("what the session's permissions do not allow is not offered", func(t *testing.T) {
		t.Parallel()

		w := newWorkspaceGranting(t, nil, "self.articles.index", "articles.show")

		definitions, err := desktop.NewResources(w, func() []resource.Definition { return []resource.Definition{articles, messages, bookmarks} }).Definitions(context.Background())

		require.NoError(t, err)
		require.Len(t, definitions, 1, "a resource none of whose listings is allowed is not offered")
		assert.Equal(t, "article", definitions[0].Name)
		assert.Equal(t, []string{"mine"}, definitions[0].Audiences)
		assert.Equal(t, desktop.Routes{List: true}, definitions[0].Mine)
		assert.Equal(t, desktop.Routes{Get: true}, definitions[0].Everyone)
	})

	t.Run("no definitions are none", func(t *testing.T) {
		t.Parallel()

		definitions, err := newResources(t, nil).Definitions(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []desktop.ResourceDefinition{}, definitions)
	})

	t.Run("a session that cannot be had offers nothing", func(t *testing.T) {
		t.Parallel()

		resources := desktop.NewResources(workspaceFailing(t, nil, auth.ErrSessionEnded), func() []resource.Definition { return []resource.Definition{articles} })

		_, err := resources.Definitions(context.Background())

		assert.ErrorIs(t, err, auth.ErrSessionEnded)
	})

	t.Run("a context whose configuration cannot be read offers nothing", func(t *testing.T) {
		t.Parallel()

		resources := desktop.NewResources(brokenWorkspace(t), func() []resource.Definition { return []resource.Definition{articles} })

		_, err := resources.Definitions(context.Background())

		assert.Error(t, err)
	})

	t.Run("a listing is laid out as its table, its numbers as they were written", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, page)
		resources := newResources(t, recorded, articles)

		listing, err := resources.List(context.Background(), "articles", true, desktop.ListQuery{Page: 2, Filters: map[string]string{"language_code": "en", "kind": ""}})

		require.NoError(t, err)
		assert.Equal(t, request{Method: http.MethodGet, Path: "/api/dashboard/articles", Query: "language_code=en&page=2", Token: "Bearer test-token"}, recorded.last(t))
		assert.Equal(t, []string{"Id", "Language", "Title", "Views"}, listing.Columns)
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
		assert.Equal(t, []string{"Uuid"}, listing.Columns)
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
		deleteErr := resources.Delete(context.Background(), "articles", true, []string{"c-1", "en"})

		assert.True(t, api.IsForbidden(listErr))
		assert.True(t, api.IsForbidden(deleteErr))
	})

	t.Run("a row's links lead where its definition says, on the context's sites", func(t *testing.T) {
		t.Parallel()

		recorded := newRecorder(http.StatusOK, `{"items":[{"correlation_uuid":"c-1","language_code":"en","published":true,"cover":"f-1"},{"correlation_uuid":"c 2","language_code":"fa"}]}`)
		w, h := newWorkspace(t, recorded)
		resources := desktop.NewResources(withWebsite(t, w, "https://web.example/"), func() []resource.Definition { return []resource.Definition{articles} })

		everybodys, everybodysErr := resources.List(context.Background(), "articles", true, desktop.ListQuery{})
		mine, mineErr := resources.List(context.Background(), "articles", false, desktop.ListQuery{})

		require.NoError(t, everybodysErr)
		require.NoError(t, mineErr)
		assert.Equal(t, map[string]string{
			"View article": "https://web.example/en/articles/c-1",
			"Edit article": "https://web.example/dashboard/articles/c-1/en",
			"Cover":        h.Server.URL + "/files/f-1",
		}, everybodys.Rows[0].Links)
		assert.Equal(t, map[string]string{
			"Edit article": "https://web.example/dashboard/articles/c 2/fa",
			"Cover":        h.Server.URL + "/files/",
		}, everybodys.Rows[1].Links, "a link a row writes nothing of is not offered for it")
		assert.NotContains(t, mine.Rows[0].Links, "Cover", "a link of everybody's is not one of one's own")
	})

	t.Run("a context with no website leads nowhere on it", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"correlation_uuid":"c-1","language_code":"en","published":true,"cover":"f-1"}]}`), articles)

		listing, err := resources.List(context.Background(), "articles", true, desktop.ListQuery{})

		require.NoError(t, err)
		assert.Equal(t, []string{"Cover"}, slices.Collect(maps.Keys(listing.Rows[0].Links)))
	})

	t.Run("a row is offered the actions that hold for it, and says whether it is deleted", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1","read":false},{"uuid":"m-2","read":true,"pinned":true}]}`), messages, articles)

		listing, err := resources.List(context.Background(), "messages", true, desktop.ListQuery{})
		articlesListing, articlesErr := resources.List(context.Background(), "articles", true, desktop.ListQuery{})

		require.NoError(t, err)
		require.NoError(t, articlesErr)
		assert.Equal(t, []string{"read", "archive"}, listing.Rows[0].Actions)
		assert.Equal(t, []string{"archive"}, listing.Rows[1].Actions)
		assert.False(t, listing.Rows[0].Protected)
		assert.True(t, listing.Rows[1].Protected)
		assert.False(t, articlesListing.Rows[0].Protected, "what says nothing of it is deleted")
	})

	t.Run("a column none of a page's rows says anything in is left out", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"correlation_uuid":"c-1","language_code":"en"},{"correlation_uuid":"c-2","language_code":"fa","title":"Hi"}]}`), articles)

		listing, err := resources.List(context.Background(), "articles", true, desktop.ListQuery{})

		require.NoError(t, err)
		assert.Equal(t, []string{"Id", "Language", "Title"}, listing.Columns)
		assert.Equal(t, []string{"c-1", "en", ""}, listing.Rows[0].Cells)
		assert.Equal(t, []string{"c-2", "fa", "Hi"}, listing.Rows[1].Cells)
	})

	t.Run("a link or an action that cannot be read or written is an error", func(t *testing.T) {
		t.Parallel()

		broken := func(name string, change func(d *resource.Definition)) resource.Definition {
			d := messages
			d.Name, d.Plural = name, name
			d.App.Links = []resource.Link{{Name: "x", Site: resource.SiteWeb, Path: "/x"}}
			change(&d)

			return d
		}

		resources := newResources(t, newRecorder(http.StatusOK, `{"items":[{"uuid":"m-1"}]}`),
			broken("link", func(d *resource.Definition) { d.App.Links[0].Path = "{{.x" }),
			broken("when", func(d *resource.Definition) {
				d.Actions = []resource.Action{{Name: "read", Title: "Read", Path: "/api/{uuid}", When: "{{.x"}}
			}),
			broken("protected", func(d *resource.Definition) { d.App.Protected = "{{.x" }),
			broken("unwritable-link", func(d *resource.Definition) { d.App.Links[0].Path = "{{.missing | upper}}" }),
			broken("unwritable-when", func(d *resource.Definition) {
				d.Actions = []resource.Action{{Name: "read", Title: "Read", Path: "/api/{uuid}", When: "{{.missing | upper}}"}}
			}),
			broken("unwritable-protected", func(d *resource.Definition) { d.App.Protected = "{{.missing | upper}}" }),
		)

		for _, name := range []string{"link", "when", "protected", "unwritable-link", "unwritable-when", "unwritable-protected"} {
			_, err := resources.List(context.Background(), name, true, desktop.ListQuery{})

			assert.Error(t, err, name)
		}
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
		deleteErr := resources.Delete(context.Background(), "messages", true, []string{"m-1"})

		assert.EqualError(t, listErr, "Bookmarks cannot be listed here")
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

		_, err := resources.Action(context.Background(), "messages", "burn", true, []string{"m-1"})

		assert.EqualError(t, err, "Messages cannot be asked to burn")
	})

	t.Run("a resource that is not described is refused", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, nil, articles)

		_, listErr := resources.List(context.Background(), "spells", true, desktop.ListQuery{})
		deleteErr := resources.Delete(context.Background(), "spells", true, nil)
		_, actionErr := resources.Action(context.Background(), "spells", "cast", true, nil)

		for _, err := range []error{listErr, deleteErr, actionErr} {
			assert.EqualError(t, err, `there is no resource called "spells"`)
		}
	})

	t.Run("a context whose configuration cannot be read is not asked", func(t *testing.T) {
		t.Parallel()

		resources := desktop.NewResources(brokenWorkspace(t), func() []resource.Definition { return []resource.Definition{articles} })

		_, listErr := resources.List(context.Background(), "articles", true, desktop.ListQuery{})
		deleteErr := resources.Delete(context.Background(), "articles", true, []string{"c-1", "en"})

		assert.Error(t, listErr)
		assert.Error(t, deleteErr)
	})

	t.Run("an empty answer is none", func(t *testing.T) {
		t.Parallel()

		resources := newResources(t, newRecorder(http.StatusOK, "  \n"), messages)

		got, err := resources.Action(context.Background(), "messages", "read", true, []string{"m-1"})

		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, "null", jsonOf(t, got))
	})
}
