package resource

import (
	"net/http"
	"strconv"
	"strings"
)

// flattened is a template of a text on one line, its line breaks spaces,
// cut to so many characters.
func flattened(field string, length int) string {
	return `{{truncate (join (split .` + field + ` "\n") " ") ` + strconv.Itoa(length) + `}}`
}

// columns is a table of one template per column.
func columns(templates ...string) string {
	return strings.Join(templates, "\t")
}

// languages is a column of a field holding something by language code, as
// "Admin (en), مدیر (fa)", each through value.
func languages(field, value string) string {
	return `{{$first := true}}{{range $code, $value := .` + field + `}}{{if $first}}{{$first = false}}{{else}}, {{end}}` + value + ` ({{$code}}){{end}}`
}

// fullName is a column of whom a row names in two fields, a first and a last
// name, either of which it may leave out.
const fullName = `{{with .first_name}}{{.}} {{end}}{{default "" .last_name}}`

// ifMoment is a template writing inner of a row whose field is a moment that
// is set: an article published, a message read.
func ifMoment(field, inner string) string {
	return `{{if ago .` + field + `}}` + inner + `{{end}}`
}

// unlessMoment is a template writing inner of a row whose field is no moment
// that is set: a message nobody read.
func unlessMoment(field, inner string) string {
	return `{{if not (ago .` + field + `)}}` + inner + `{{end}}`
}

// edit is a link to the page of the web dashboard a row is changed on,
// offered where reading and changing it are allowed, as the dashboard
// offers it.
func edit(name, path string, audiences ...Audience) Link {
	return Link{Name: name, Kind: LinkEdit, Site: SiteWeb, Path: path, Requires: []Operation{OperationGet, OperationUpdate}, Audiences: audiences}
}

// show is a link to the page of the web dashboard a row is read on,
// offered where reading it is allowed.
func show(name, path string, audiences ...Audience) Link {
	return Link{Name: name, Kind: LinkView, Site: SiteWeb, Path: path, Requires: []Operation{OperationGet}, Audiences: audiences}
}

// public is a link to a public page of the website, which anybody opens.
func public(name string, kind LinkKind, path string, audiences ...Audience) Link {
	return Link{Name: name, Kind: kind, Site: SiteWeb, Path: path, Audiences: audiences}
}

// articlePage is the path of the public page of the article a row names by
// its correlation uuid and language code fields.
func articlePage(uuid, language string) string {
	return `/{{.` + language + `}}/articles/{{.` + uuid + `}}`
}

// content are what is published: articles and what hangs off them.
var content = []Definition{
	{
		Name:        "article",
		Plural:      "articles",
		Title:       "Articles",
		Description: "Articles, each in the languages it is written in: a version per language, keyed by the correlation uuid the versions share and its language code.",
		Section:     SectionContent,
		Key:         []string{"correlation_uuid", "language_code"},
		Everyone: Paths{
			List:   "/api/dashboard/articles",
			Get:    "/api/dashboard/articles/{correlation_uuid}/{language_code}",
			Create: "/api/dashboard/articles",
			Update: "/api/dashboard/articles",
			Delete: "/api/dashboard/articles/{correlation_uuid}/{language_code}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/articles",
			Get:    "/api/dashboard/my/articles/{correlation_uuid}/{language_code}",
			Update: "/api/dashboard/my/articles",
			Delete: "/api/dashboard/my/articles/{correlation_uuid}/{language_code}",
		},
		Versions: "corrolated_items",
		Table:    columns(`{{.correlation_uuid}}`, `{{.language.code}}`, flattened("title", 50), `{{.author.name}}`, `{{ago .published_at}}`),
		Headers:  map[string]string{"code": "LANGUAGE", "name": "AUTHOR", "published_at": "PUBLISHED"},
		ID:       `{{.correlation_uuid}}/{{.language.code}}`,
		App: App{
			Table: columns(flattened("title", 60), `{{.language.code}}`, `{{.author.name}}`, `{{default "Not published" (ago .published_at)}}`),
			New:   New{Name: "New article", Path: "/dashboard/articles/new"},
			Links: []Link{
				public("View article", LinkView, ifMoment("published_at", articlePage("correlation_uuid", "language.code"))),
				edit("Edit article", `/dashboard/articles/{{.correlation_uuid}}/{{.language.code}}`),
			},
		},
	},
	{
		Name:        "comment",
		Plural:      "comments",
		Title:       "Comments",
		Description: "Comments on articles, shown once they are approved.",
		Section:     SectionContent,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/comments",
			Get:    "/api/dashboard/comments/{uuid}",
			Create: "/api/dashboard/comments",
			Update: "/api/dashboard/comments",
			Delete: "/api/dashboard/comments/{uuid}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/comments",
			Get:    "/api/dashboard/my/comments/{uuid}",
			Update: "/api/dashboard/my/comments",
			Delete: "/api/dashboard/my/comments/{uuid}",
		},
		Defaults: map[string]any{"object_type": "article"},
		Table:    columns(`{{.uuid}}`, flattened("body", 40), `{{with .author}}{{.name}}{{end}}`, `{{.object_uuid}}`, `{{.language_code}}`, `{{ago .approved_at}}`, `{{ago .created_at}}`),
		Headers:  map[string]string{"object_uuid": "ARTICLE", "language_code": "LANGUAGE", "approved_at": "APPROVED", "created_at": "CREATED"},
		App: App{
			Table: columns(flattened("body", 60), `{{with .author}}{{.name}}{{end}}`, `{{default "Not approved" (ago .approved_at)}}`, `{{ago .created_at}}`),
			Links: []Link{
				public("View comment", LinkView, articlePage("object_uuid", "language_code")),
				edit("Edit comment", `/dashboard/comments/{{.uuid}}`, AudienceEveryone),
			},
		},
	},
	{
		Name:        "file",
		Plural:      "files",
		Title:       "Files",
		Description: "Files uploaded to be linked to, no larger than the uploader's subscription allows.",
		Section:     SectionContent,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/files",
			Delete: "/api/dashboard/files/{uuid}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/files",
			Delete: "/api/dashboard/my/files/{uuid}",
		},
		Table:   columns(`{{.uuid}}`, `{{.name}}`, `{{bytes .size}}`, `{{.mimetype}}`, `{{.owner_uuid}}`),
		Headers: map[string]string{"mimetype": "TYPE", "owner_uuid": "OWNER"},
		App: App{
			Table: columns(`{{.name}}`, `{{bytes .size}}`, `{{.mimetype}}`),
			Links: []Link{{Name: "View file", Kind: LinkView, Site: SiteAPI, Path: `/files/{{.uuid}}`}},
		},
	},
	{
		Name:        "element",
		Plural:      "elements",
		Title:       "Elements",
		Description: "Widgets shown on the pages whose paths match their venues, each pointing at articles.",
		Section:     SectionContent,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/elements",
			Get:    "/api/dashboard/elements/{uuid}",
			Create: "/api/dashboard/elements",
			Update: "/api/dashboard/elements",
			Delete: "/api/dashboard/elements/{uuid}",
		},
		Table:   columns(`{{.uuid}}`, `{{.body_type}}`, `{{join .venues ", "}}`, `{{ago .updated_at}}`),
		Headers: map[string]string{"body_type": "TYPE", "updated_at": "UPDATED"},
		App: App{
			Table: columns(`{{.body_type}}`, `{{join .venues ", "}}`, `{{ago .updated_at}}`),
			New:   New{Name: "New element", Path: "/dashboard/elements/new"},
			Links: []Link{edit("Edit element", `/dashboard/elements/{{.uuid}}`)},
		},
	},
	{
		Name:        "contact",
		Plural:      "contact-us",
		Title:       "Contact messages",
		Description: "Messages sent through the contact form.",
		Section:     SectionContent,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/contact-us",
			Get:    "/api/dashboard/contact-us/{uuid}",
			Delete: "/api/dashboard/contact-us/{uuid}",
		},
		Actions: []Action{
			{
				Name: "read", Description: "Mark one or more messages read", Method: http.MethodPut, Path: "/api/dashboard/contact-us/{uuid}/read", Body: map[string]any{"read": true},
				Title: "Mark as read", When: unlessMoment("read_at", "true"),
			},
			{
				Name: "unread", Description: "Mark one or more messages unread", Method: http.MethodPut, Path: "/api/dashboard/contact-us/{uuid}/read", Body: map[string]any{"read": false},
				Title: "Mark as unread", When: ifMoment("read_at", "true"),
			},
		},
		Table:   columns(`{{.uuid}}`, flattened("subject", 40), `{{default "" .email}}`, `{{default "" .phone}}`, `{{ago .read_at}}`, `{{ago .created_at}}`),
		Headers: map[string]string{"read_at": "READ", "created_at": "RECEIVED"},
		App: App{
			Table: columns(flattened("subject", 50), `{{default "" .email}}`, `{{default "" .phone}}`, `{{ago .created_at}}`, `{{default "Unread" (ago .read_at)}}`),
			Links: []Link{show("View message", `/dashboard/contact-us/{{.uuid}}`)},
		},
	},
}

// learning are exams, what they ask, and the licenses passing them issues.
var learning = []Definition{
	{
		Name:        "question",
		Plural:      "questions",
		Title:       "Questions",
		Description: "Questions exams draw from: a choice of one or more options, a text, or code an examiner runs.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/questions",
			Get:    "/api/dashboard/questions/{uuid}",
			Create: "/api/dashboard/questions",
			Update: "/api/dashboard/questions",
			Delete: "/api/dashboard/questions/{uuid}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/questions",
			Get:    "/api/dashboard/my/questions/{uuid}",
			Update: "/api/dashboard/my/questions",
			Delete: "/api/dashboard/my/questions/{uuid}",
		},
		Filters: []Filter{
			{Name: "language_code", Description: "only the questions in a language"},
			{Name: "kind", Description: "only the questions of a kind", Values: []string{"single_choice", "multiple_choice", "text", "code"}},
		},
		Table:   columns(`{{.uuid}}`, `{{.kind}}`, `{{.language_code}}`, `{{default "" .difficulty}}`, flattened("prompt", 50)),
		Headers: map[string]string{"language_code": "LANGUAGE"},
		App: App{
			Table: columns(flattened("prompt", 60), `{{.kind}}`, `{{default "" .difficulty}}`, `{{.language_code}}`),
			New:   New{Name: "New question", Path: "/dashboard/questions/new"},
			Links: []Link{edit("Edit", `/dashboard/questions/{{.uuid}}`)},
		},
	},
	{
		Name:        "exam",
		Plural:      "exams",
		Title:       "Exams",
		Description: "Exams of stages, each drawing its questions at random, passed at a score and issuing a license from a template.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/exams",
			Get:    "/api/dashboard/exams/{uuid}",
			Create: "/api/dashboard/exams",
			Update: "/api/dashboard/exams",
			Delete: "/api/dashboard/exams/{uuid}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/exams",
			Get:    "/api/dashboard/my/exams/{uuid}",
			Update: "/api/dashboard/my/exams",
			Delete: "/api/dashboard/my/exams/{uuid}",
		},
		Filters: []Filter{
			{Name: "language_code", Description: "only the exams in a language"},
		},
		Table:   columns(`{{.uuid}}`, `{{.language_code}}`, flattened("title", 40), `{{.stages}}`, `{{.passing_score}}%`, `{{.attempts.total}}`, `{{.attempts.in_progress}}`, `{{.attempts.passed}}`),
		Headers: map[string]string{"language_code": "LANGUAGE", "passing_score": "PASSING SCORE", "total": "ATTEMPTS"},
		App: App{
			Table: columns(flattened("title", 50), `{{.language_code}}`, `{{.stages}}`, `{{.passing_score}}%`, `{{.attempts.total}}`, `{{.attempts.passed}}`),
			New:   New{Name: "New exam", Path: "/dashboard/exams/new"},
			Links: []Link{
				edit("Edit", `/dashboard/exams/{{.uuid}}`),
				{Name: "Attempts", Kind: LinkList, Site: SiteApp, Resource: "exam-attempt", Filter: "exam_uuid", Path: `{{.uuid}}`},
				public("Public page", LinkPublic, `/{{.language_code}}/exams/{{.uuid}}`),
			},
		},
	},
	{
		Name:        "attendable-exam",
		Plural:      "attendable-exams",
		Title:       "Attendable exams",
		Description: "The exams one may attempt, or is attempting, as one's roles allow attending them.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Mine: Paths{
			List: "/api/dashboard/attendable-exams",
			Get:  "/api/exams/{uuid}",
		},
		Filters: []Filter{
			{Name: "language_code", Description: "only the exams in a language"},
		},
		Table:   columns(`{{.uuid}}`, `{{.language_code}}`, flattened("title", 40), `{{.questions}}`, `{{.passing_score}}%`, `{{default "unlimited" .attempts_left}}`, `{{default "" .in_progress_uuid}}`),
		Headers: map[string]string{"language_code": "LANGUAGE", "passing_score": "PASSING SCORE", "in_progress_uuid": "IN PROGRESS"},
		App: App{
			Table: columns(flattened("title", 50), `{{.language_code}}`, `{{.questions}}`, `{{.passing_score}}%`, `{{default "unlimited" .attempts_left}}`, `{{if .in_progress_uuid}}yes{{end}}`),
			Links: []Link{
				public("Continue your attempt", LinkPublic, `{{with .in_progress_uuid}}/{{$.language_code}}/exams/attempts/{{.}}{{end}}`),
				public("Start the exam", LinkPublic, `{{with .in_progress_uuid}}{{else}}/{{.language_code}}/exams/{{.uuid}}{{end}}`),
			},
		},
	},
	{
		Name:        "exam-attempt",
		Plural:      "exam-attempts",
		Title:       "Exam attempts",
		Description: "Attempts made at exams, as they were asked and answered, graded once every answer is evaluated.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List: "/api/dashboard/exam-attempts",
			Get:  "/api/dashboard/exam-attempts/{uuid}",
		},
		Mine: Paths{
			List: "/api/dashboard/my/exam-attempts",
			Get:  "/api/dashboard/my/exam-attempts/{uuid}",
		},
		Actions: []Action{
			{
				Name: "finish", Description: "Finish one or more of one's attempts before they are all answered or their time is up", Method: http.MethodPost, Path: "/api/exam-attempts/{uuid}/finish", Mine: true,
				Title:   "Finish attempt",
				Confirm: "Finish this attempt? The questions you haven't answered stay unanswered, and the attempt is graded on what you did answer. You can't go back to it.",
				When:    `{{eq .status "in_progress"}}`,
			},
		},
		Filters: []Filter{
			{Name: "exam_uuid", Description: "only the attempts at an exam"},
			{Name: "user_uuid", Description: "only the attempts somebody made", Audiences: []Audience{AudienceEveryone}},
		},
		Table:   columns(`{{.uuid}}`, flattened("exam_title", 30), fullName, `{{.status}}`, `{{default "" .score}}`, `{{.passing_score}}`, `{{ago .started_at}}`, `{{ago .finished_at}}`),
		Headers: map[string]string{"exam_title": "EXAM", "first_name": "NAME", "passing_score": "PASSING SCORE", "started_at": "STARTED", "finished_at": "FINISHED"},
		App: App{
			Table: columns(flattened("exam_title", 40), fullName, `{{.status}}`, `{{default "" .score}}`, `{{ago .started_at}}`, `{{ago .finished_at}}`),
			Links: []Link{
				show("View", `/dashboard/exam-attempts/{{.uuid}}`, AudienceEveryone),
				public("Resume", LinkPublic, `{{if eq .status "in_progress"}}/{{.language_code}}/exams/attempts/{{.uuid}}{{end}}`, AudienceMine),
				show("View", `{{if ne .status "in_progress"}}/dashboard/exam-attempts/{{.uuid}}{{end}}`, AudienceMine),
				public("View your license", LinkPublic, `{{with .license_uuid}}/license/{{.}}{{end}}`, AudienceMine),
			},
		},
	},
	{
		Name:        "license-template",
		Plural:      "license-templates",
		Title:       "License templates",
		Description: "What the licenses an exam issues say, and for how long they are valid.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/license-templates",
			Get:    "/api/dashboard/license-templates/{uuid}",
			Create: "/api/dashboard/license-templates",
			Update: "/api/dashboard/license-templates",
			Delete: "/api/dashboard/license-templates/{uuid}",
		},
		Mine: Paths{
			List:   "/api/dashboard/my/license-templates",
			Get:    "/api/dashboard/my/license-templates/{uuid}",
			Update: "/api/dashboard/my/license-templates",
			Delete: "/api/dashboard/my/license-templates/{uuid}",
		},
		Table:   columns(`{{.uuid}}`, flattened("title", 40), `{{.validity_seconds}}`, `{{ago .updated_at}}`),
		Headers: map[string]string{"updated_at": "UPDATED"},
		App: App{
			Table: columns(flattened("title", 50), `{{default "Forever" (duration .validity_seconds)}}`, `{{ago .updated_at}}`),
			New:   New{Name: "New license template", Path: "/dashboard/license-templates/new"},
			Links: []Link{edit("Edit", `/dashboard/license-templates/{{.uuid}}`)},
		},
	},
	{
		Name:        "license",
		Plural:      "licenses",
		Title:       "Licenses",
		Description: "Licenses issued for passing exams, or by hand, readable by anybody given their link.",
		Section:     SectionLearning,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/licenses",
			Get:    "/api/dashboard/licenses/{uuid}",
			Create: "/api/dashboard/licenses",
			Delete: "/api/dashboard/licenses/{uuid}",
		},
		Mine: Paths{
			List: "/api/dashboard/my/licenses",
			Get:  "/api/licenses/{uuid}",
		},
		Filters: []Filter{
			{Name: "user_uuid", Description: "only the licenses somebody holds", Audiences: []Audience{AudienceEveryone}},
			{Name: "exam_uuid", Description: "only the licenses an exam issued", Audiences: []Audience{AudienceEveryone}},
		},
		Table:   columns(`{{.uuid}}`, flattened("title", 40), `{{.topic}}`, fullName, `{{ago .issued_at}}`, `{{ago .expires_at}}`, `{{.expired}}`),
		Headers: map[string]string{"first_name": "HOLDER", "issued_at": "ISSUED", "expires_at": "EXPIRES"},
		App: App{
			Table: columns(fullName, `{{.topic}}`, flattened("title", 40), `{{ago .issued_at}}`, `{{default "Never" (ago .expires_at)}}`),
			New:   New{Name: "Issue a license", Path: "/dashboard/licenses/new"},
			Links: []Link{
				show("View", `/dashboard/licenses/{{.uuid}}`, AudienceEveryone),
				public("View license", LinkPublic, `/license/{{.uuid}}`, AudienceMine),
			},
		},
	},
}

// administration are who may do what, and what everybody did.
var administration = []Definition{
	{
		Name:        "user",
		Plural:      "users",
		Title:       "Users",
		Description: "The accounts that sign in, with the roles and the subscription they are given.",
		Section:     SectionAdministration,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/users",
			Get:    "/api/dashboard/users/{uuid}",
			Create: "/api/dashboard/users",
			Update: "/api/dashboard/users",
			Delete: "/api/dashboard/users/{uuid}",
		},
		Actions: []Action{
			{Name: "signout", Description: "Sign one or more users out of every session but the one asking", Method: http.MethodDelete, Path: "/api/dashboard/users/{uuid}/sessions"},
		},
		Table: columns(`{{.uuid}}`, `{{default "" .username}}`, `{{default "" .name}}`, `{{default "" .email}}`),
		App: App{
			Table: columns(`{{default "" .name}}`, `{{default "" .username}}`, `{{default "" .email}}`),
			New:   New{Name: "New user", Path: "/dashboard/users/new"},
			Links: []Link{edit("Edit user", `/dashboard/users/{{.uuid}}`)},
		},
	},
	{
		Name:        "role",
		Plural:      "roles",
		Title:       "Roles",
		Description: "What their holders may do: statements allowing or disallowing actions on resources, a disallow outweighing every allow.",
		Section:     SectionAdministration,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/roles",
			Get:    "/api/dashboard/roles/{uuid}",
			Create: "/api/dashboard/roles",
			Update: "/api/dashboard/roles",
			Delete: "/api/dashboard/roles/{uuid}",
		},
		Mine: Paths{
			List: "/api/dashboard/profile/roles",
		},
		Table:   columns(`{{.uuid}}`, languages("translations", "{{$value.title}}"), `{{if .is_default}}yes{{end}}`),
		Headers: map[string]string{"translations": "TITLE", "is_default": "DEFAULT"},
		App: App{
			Audiences: []Audience{AudienceEveryone},
			Table:     columns(languages("translations", "{{$value.title}}"), `{{if .is_default}}yes{{end}}`),
			New:       New{Name: "New role", Path: "/dashboard/roles/new"},
			Links:     []Link{edit("Edit role", `/dashboard/roles/{{.uuid}}`)},
			Protected: `{{.is_default}}`,
		},
	},
	{
		Name:        "subscription",
		Plural:      "subscriptions",
		Title:       "Subscriptions",
		Description: "What their holders may have, in VMs, snapshots and uploads, and the roles they hold while on one.",
		Section:     SectionAdministration,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List:   "/api/dashboard/subscriptions",
			Get:    "/api/dashboard/subscriptions/{uuid}",
			Create: "/api/dashboard/subscriptions",
			Update: "/api/dashboard/subscriptions",
			Delete: "/api/dashboard/subscriptions/{uuid}",
		},
		Table: columns(
			`{{.uuid}}`,
			languages("names", "{{$value}}"),
			`{{with .quotas.vms}}{{.max}}{{else}}unlimited{{end}}`,
			`{{with .quotas.snapshots}}{{.max}}{{else}}unlimited{{end}}`,
			`{{with .quotas.uploads}}{{bytes .max_size}}{{else}}unlimited{{end}}`,
			`{{if .is_default}}yes{{end}}`,
		),
		Headers: map[string]string{"names": "NAME", "uploads": "UPLOAD SIZE", "is_default": "DEFAULT"},
		App: App{
			Table: columns(
				languages("names", "{{$value}}"),
				`{{with .quotas.vms}}{{.max}}{{else}}unlimited{{end}}`,
				`{{with .quotas.snapshots}}{{.max}}{{else}}unlimited{{end}}`,
				`{{with .quotas.uploads}}{{bytes .max_size}}{{else}}unlimited{{end}}`,
				`{{if .is_default}}yes{{end}}`,
			),
			New:       New{Name: "New subscription", Path: "/dashboard/subscriptions/new"},
			Links:     []Link{edit("Edit subscription", `/dashboard/subscriptions/{{.uuid}}`)},
			Protected: `{{.is_default}}`,
		},
	},
	{
		Name:        "language",
		Plural:      "languages",
		Title:       "Languages",
		Description: "The languages content is written in, one of them the default.",
		Section:     SectionAdministration,
		Key:         []string{"code"},
		Everyone: Paths{
			List:   "/api/dashboard/languages",
			Get:    "/api/dashboard/languages/{code}",
			Create: "/api/dashboard/languages",
			Update: "/api/dashboard/languages",
			Delete: "/api/dashboard/languages/{code}",
		},
		Table:   columns(`{{.code}}`, `{{.name}}`, `{{if .is_default}}yes{{end}}`),
		Headers: map[string]string{"is_default": "DEFAULT"},
		App: App{
			New:       New{Name: "New language", Path: "/dashboard/languages/new"},
			Links:     []Link{edit("Edit language", `/dashboard/languages/{{.code}}`)},
			Protected: `{{.is_default}}`,
		},
	},
	{
		Name:        "permission",
		Plural:      "permissions",
		Title:       "Permissions",
		Description: "Every action a role's statements may allow or disallow.",
		Section:     SectionAdministration,
		Key:         []string{"value"},
		Everyone: Paths{
			List: "/api/dashboard/permissions",
		},
		Table:   columns(`{{.value}}`, `{{.name}}`),
		Headers: map[string]string{"value": "PERMISSION", "name": "DESCRIPTION"},
		App:     App{Hidden: true},
	},
	{
		Name:        "audit-log",
		Plural:      "audit-logs",
		Title:       "Audit logs",
		Description: "Every action taken through the API: who took it, on what, from where, and what it was answered.",
		Section:     SectionAdministration,
		Key:         []string{"uuid"},
		Everyone: Paths{
			List: "/api/dashboard/audit-logs",
		},
		Mine: Paths{
			List: "/api/dashboard/my/audit-logs",
		},
		Actions: []Action{
			{Name: "activity", Description: "Count the actions taken a step at a time", Method: http.MethodGet, Path: "/api/dashboard/audit-logs/activity"},
			{Name: "activity", Description: "Count the actions taken a step at a time", Method: http.MethodGet, Path: "/api/dashboard/my/audit-logs/activity", Mine: true},
		},
		Filters: []Filter{
			{Name: "kind", Description: "only the actions of a kind", Values: []string{"read", "create", "update", "delete"}},
			{Name: "ip", Description: "only what was done from an address"},
			{Name: "from", Description: "only what was done at this moment or later", Moment: true},
			{Name: "to", Description: "only what was done at this moment or earlier", Moment: true},
			{Name: "user_uuid", Description: "only what somebody did", Audiences: []Audience{AudienceEveryone}},
		},
		Cursor:  "before",
		Table:   columns(`{{ago .occurred_at}}`, `{{.kind}}`, `{{.action}}`, `{{default "" .entity.id}}`, `{{.status}}`, `{{with .user}}{{default .uuid .username}}{{end}}`, `{{default "" .ip}}`),
		Headers: map[string]string{"occurred_at": "WHEN", "id": "ENTITY", "user": "USER"},
		App: App{
			Table: columns(`{{ago .occurred_at}}`, `{{.kind}}`, `{{.action}}`, `{{.status}}`, `{{with .user}}{{default (default "" .username) .name}}{{end}}`, `{{default "" .ip}}`),
		},
	},
}

// account are what is about the one signed in.
var account = []Definition{
	{
		Name:        "bookmark",
		Plural:      "bookmarks",
		Title:       "Bookmarks",
		Description: "The articles one bookmarked, a language version each.",
		Section:     SectionAccount,
		Key:         []string{"object_uuid", "language_code"},
		Mine: Paths{
			List:   "/api/dashboard/my/bookmarks",
			Delete: "/api/dashboard/my/bookmarks",
		},
		Defaults: map[string]any{"object_type": "article"},
		Table:    columns(`{{.object_uuid}}`, `{{.language_code}}`, flattened("title", 50), `{{ago .created_at}}`),
		Headers:  map[string]string{"object_uuid": "ARTICLE", "language_code": "LANGUAGE", "created_at": "CREATED"},
		App: App{
			Table: columns(flattened("title", 60), `{{.language_code}}`, `{{ago .created_at}}`),
			Links: []Link{public("View bookmark", LinkView, articlePage("object_uuid", "language_code"))},
		},
	},
	{
		Name:        "session",
		Plural:      "sessions",
		Title:       "Sessions",
		Description: "Where one is signed in: every token is issued in a session, and a token whose session is gone is refused at once.",
		Section:     SectionAccount,
		Key:         []string{"uuid"},
		Mine: Paths{
			List:   "/api/dashboard/profile/sessions",
			Delete: "/api/dashboard/profile/sessions/{uuid}",
		},
		Actions: []Action{
			{
				Name: "rm-others", Description: "Sign out of every session but this one", Method: http.MethodDelete, Path: "/api/dashboard/profile/sessions/others", Mine: true,
				Title: "Terminate all other sessions", Confirm: "Terminate every other session? Only this device stays signed in.",
			},
		},
		Table: columns(
			`{{.uuid}}`,
			`{{if .current}}yes{{end}}`,
			`{{with .client}}{{default .id .name}}{{end}}`,
			`{{default "" .device.browser}}`,
			`{{default "" .device.os}}`,
			`{{with .impersonated_user}}{{default .uuid .username}}{{end}}`,
			`{{default "" .ip}}`,
			`{{ago .last_activity.at}}`,
			`{{ago .created_at}}`,
		),
		Headers: map[string]string{"impersonated_user": "AS", "at": "LAST ACTIVE", "created_at": "CREATED"},
		App: App{
			Table: columns(
				`{{if .current}}yes{{end}}`,
				`{{with .client}}{{default .id .name}}{{end}}`,
				`{{default "" .device.browser}}`,
				`{{default "" .device.os}}`,
				`{{with .impersonated_user}}{{default (default "" .username) .name}}{{end}}`,
				`{{default "" .ip}}`,
				`{{ago .last_activity.at}}`,
				`{{ago .created_at}}`,
			),
		},
	},
}
