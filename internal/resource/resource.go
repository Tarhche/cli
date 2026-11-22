// Package resource describes what the dashboard manages, as the API serves
// it: where each kind of thing is listed, read, created, changed and
// deleted, what identifies one, and how a listing of them is shown. The
// CLI's generic commands and the desktop app's generic pages are both built
// from it.
package resource

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"
)

// Sections group resources in the desktop app's navigation.
const (
	SectionContent        = "Content"
	SectionLearning       = "Learning"
	SectionAdministration = "Administration"
	SectionAccount        = "Account"
)

// Audience is whose things a route serves.
type Audience string

const (
	// AudienceEveryone is everybody's things, which an administrator
	// manages, under /api/dashboard/....
	AudienceEveryone Audience = "everyone"

	// AudienceMine is one's own things, under /api/dashboard/my/....
	AudienceMine Audience = "mine"
)

// Operation is one of the five things a resource may be asked.
type Operation string

// The operations, each served at the route of its name (see Paths).
const (
	OperationList   Operation = "list"
	OperationGet    Operation = "get"
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
	OperationDelete Operation = "delete"
)

// ErrInvalidKey is a key that is not one of the resource's.
var ErrInvalidKey = errors.New("invalid key")

// noValue is what a template writes of a field an item does not have.
const noValue = "<no value>"

// Paths are the routes of one audience of a resource: everybody's, which an
// administrator manages, or one's own. A route that is empty is not served.
// Get and Delete name the resource's key fields in braces, as
// "/api/dashboard/articles/{correlation_uuid}/{language_code}"; the key
// fields a route does not name in braces are sent in its body, as an
// update's are.
type Paths struct {
	List   string
	Get    string
	Create string
	Update string
	Delete string
}

// Route is the path op is served at, empty when it is not served.
func (p Paths) Route(op Operation) string {
	switch op {
	case OperationList:
		return p.List
	case OperationGet:
		return p.Get
	case OperationCreate:
		return p.Create
	case OperationUpdate:
		return p.Update
	case OperationDelete:
		return p.Delete
	default:
		return ""
	}
}

// served reports whether any route is.
func (p Paths) served() bool {
	return p != Paths{}
}

// Action is something more than reading, creating, changing and deleting
// that a resource can be asked: a contact message marked read, say. It
// takes nothing but the key of what it is asked of, if its path names one.
type Action struct {
	// Name is the command it is offered as: "read".
	Name        string
	Description string

	// Title is what the desktop app offers it as: "Mark as read". The app
	// offers none that has no title, which is the CLI's alone.
	Title string

	// Confirm is what the app asks before it asks the API, nothing for
	// nothing.
	Confirm string

	// When is a template over a row that writes "true" of the rows the app
	// offers it for; every row when empty.
	When string

	Method string
	// Path names the key fields in braces, as Paths.Get does.
	Path string

	// Body is what it is sent with, nil for nothing.
	Body map[string]any

	// Mine says it is one of one's own routes rather than everybody's.
	Mine bool
}

// Audience is whose things the action is asked of.
func (a Action) Audience() Audience {
	if a.Mine {
		return AudienceMine
	}

	return AudienceEveryone
}

// Site is where a link leads.
type Site string

const (
	// SiteWeb is the estate's website: its public pages and its web
	// dashboard.
	SiteWeb Site = "web"

	// SiteAPI is what the API serves itself, as the files it keeps.
	SiteAPI Site = "api"

	// SiteApp is another page of the desktop app: everybody's listing of a
	// resource, narrowed by one of its filters.
	SiteApp Site = "app"
)

// LinkKind is what a link does, which the app shows it by.
type LinkKind string

const (
	// LinkView shows what a row is.
	LinkView LinkKind = "view"

	// LinkEdit changes it.
	LinkEdit LinkKind = "edit"

	// LinkList lists what belongs to it.
	LinkList LinkKind = "list"

	// LinkPublic is its public page, beside the one that shows it.
	LinkPublic LinkKind = "public"
)

// Link is a page a row of a resource is shown or changed on beside the
// app's listing of it: a page of the estate's website, something its API
// serves, or a page of the app. The desktop app offers its links as the web
// dashboard's listing offers them, a page of the website opened in the
// browser.
type Link struct {
	// Name is what it is offered as: "Edit article".
	Name string
	Kind LinkKind
	Site Site

	// Path is a template over a row of where it leads: a path on its site,
	// or, on the app's, the value of Filter. A row it writes nothing of has
	// no such page.
	Path string

	// Resource is the resource whose listing a link of the app opens, and
	// Filter the filter Path writes the value of.
	Resource string
	Filter   string

	// Requires are what the session has to be allowed of the audience it is
	// offered in, as the web dashboard asks before it offers a page: an
	// edit page needs reading and changing. None is a page anybody opens.
	Requires []Operation

	// Audiences are the listings it is offered in; none is every one.
	Audiences []Audience
}

// OfferedTo reports whether it is offered in the listing of an audience.
func (l Link) OfferedTo(audience Audience) bool {
	return len(l.Audiences) == 0 || slices.Contains(l.Audiences, audience)
}

// New is the page of the web dashboard one of a resource is made on.
type New struct {
	// Name is what it is offered as: "New article".
	Name string

	// Path is where it is on the website.
	Path string
}

// App is how the desktop app shows a resource where that is not what the
// CLI does with it: what a person reads, and where they go to do what the
// app does not, rather than what a command is given.
type App struct {
	// Hidden says the app has no page of it.
	Hidden bool

	// Audiences are whose things its page lists, of those it is served to;
	// every one when none.
	Audiences []Audience

	// Table is its page's table, over a row as Table is, Table when empty:
	// no uuid, as a person tells rows apart by what they say.
	Table string

	// New is where one is made, nowhere when it has no name.
	New New

	// Links are where a row is shown or changed, in the order they are
	// offered, before its actions and its delete.
	Links []Link

	// Protected is a template over a row that writes "true" of one that is
	// not deleted: the default role.
	Protected string
}

// Filter is a query parameter a listing takes to narrow what it lists.
type Filter struct {
	// Name is the query parameter: "language_code".
	Name        string
	Description string

	// Values are what it takes, when it takes only some values.
	Values []string

	// Moment says it takes a moment, in RFC 3339.
	Moment bool

	// Audiences are the listings that take it; none is every one.
	Audiences []Audience
}

// Definition is a kind of thing the dashboard manages.
type Definition struct {
	// Name is the singular the CLI's group is named after: "article".
	Name string
	// Plural is how the API names more than one: "articles".
	Plural string
	// Title is what the desktop app calls them: "Articles".
	Title       string
	Description string
	Section     string

	// Key are the fields that identify one, in the order a command is given
	// them: ["uuid"], or ["correlation_uuid", "language_code"]. A command
	// is given a key as its fields joined by slashes.
	Key []string

	Everyone Paths
	Mine     Paths

	Actions []Action

	// Filters are the query parameters its listings take.
	Filters []Filter

	// Cursor is the query parameter a listing paged by a cursor is asked
	// for its next page with, as the "next" a page answers names it:
	// "before". Empty for a listing paged by number.
	Cursor string

	// Versions is the field of a listed item holding the versions it is
	// listed as, when a listing is of each of them rather than of the item:
	// an article's languages, in "corrolated_items". See Rows.
	Versions string

	// Defaults are the fields a body sent to it carries when it does not
	// say them itself: a bookmark's object_type.
	Defaults map[string]any

	// Table is a listing's default table, one template per column separated
	// by tabs, over a row (see Rows).
	Table string

	// Headers name the columns whose header is not their field's name in
	// capitals, by field.
	Headers map[string]string

	// ID is the template of what identifies a row, as its key is given to a
	// command; the key fields joined by slashes when empty.
	ID string

	// App is how the desktop app shows it.
	App App
}

// Path is path with the key fields in braces replaced by key, in order.
func (d Definition) Path(path string, key []string) string {
	for i, field := range d.Key {
		if i < len(key) {
			path = strings.ReplaceAll(path, "{"+field+"}", key[i])
		}
	}

	return path
}

// BodyKey are the fields of key that path does not name in braces, by
// field: what a request to it carries in its body instead.
func (d Definition) BodyKey(path string, key []string) map[string]any {
	fields := make(map[string]any)

	for i, field := range d.Key {
		if i < len(key) && !strings.Contains(path, "{"+field+"}") {
			fields[field] = key[i]
		}
	}

	return fields
}

// PathsOf are the routes of an audience.
func (d Definition) PathsOf(audience Audience) Paths {
	if audience == AudienceMine {
		return d.Mine
	}

	return d.Everyone
}

// Audiences are the audiences it is served to, one's own first, as commands
// act on one's own things unless asked for everybody's.
func (d Definition) Audiences() []Audience {
	var audiences []Audience

	if d.Mine.served() || slices.ContainsFunc(d.Actions, func(a Action) bool { return a.Mine }) {
		audiences = append(audiences, AudienceMine)
	}

	if d.Everyone.served() || slices.ContainsFunc(d.Actions, func(a Action) bool { return !a.Mine }) {
		audiences = append(audiences, AudienceEveryone)
	}

	return audiences
}

// AppAudiences are the audiences the desktop app lists it to, of those it
// is served to: none when the app has no page of it.
func (d Definition) AppAudiences() []Audience {
	if d.App.Hidden {
		return nil
	}

	var audiences []Audience

	for _, audience := range d.Audiences() {
		if len(d.App.Audiences) == 0 || slices.Contains(d.App.Audiences, audience) {
			audiences = append(audiences, audience)
		}
	}

	return audiences
}

// AppTable is the desktop app's table of a listing: its own, else Table.
func (d Definition) AppTable() string {
	if len(d.App.Table) > 0 {
		return d.App.Table
	}

	return d.Table
}

// Serves reports whether op is served to any audience.
func (d Definition) Serves(op Operation) bool {
	return len(d.Mine.Route(op)) > 0 || len(d.Everyone.Route(op)) > 0
}

// ServesBoth reports whether op is served for one's own things and for
// everybody's, so that whoever asks chooses whose.
func (d Definition) ServesBoth(op Operation) bool {
	return len(d.Mine.Route(op)) > 0 && len(d.Everyone.Route(op)) > 0
}

// Route is the path op is served at, and whose things it serves: one's own
// unless everyone is asked for or one's own are not served, everybody's
// otherwise. It is false when op is not served that way.
func (d Definition) Route(op Operation, everyone bool) (string, Audience, bool) {
	if !everyone {
		if path := d.Mine.Route(op); len(path) > 0 {
			return path, AudienceMine, true
		}
	}

	if path := d.Everyone.Route(op); len(path) > 0 {
		return path, AudienceEveryone, true
	}

	return "", "", false
}

// FiltersOf are the filters the listing of an audience takes.
func (d Definition) FiltersOf(audience Audience) []Filter {
	var filters []Filter

	for _, filter := range d.Filters {
		if len(filter.Audiences) == 0 || slices.Contains(filter.Audiences, audience) {
			filters = append(filters, filter)
		}
	}

	return filters
}

// KeyUsage is how a key is given to a command: its fields in capitals,
// joined by slashes, as "CORRELATION_UUID/LANGUAGE_CODE".
func (d Definition) KeyUsage() string {
	return strings.ToUpper(strings.Join(d.Key, "/"))
}

// ParseKey is the key a command was given: as many fields as the key has,
// joined by slashes, none of them empty, "." or "..".
func (d Definition) ParseKey(given string) ([]string, error) {
	fields := strings.Split(given, "/")

	if len(fields) != len(d.Key) || slices.ContainsFunc(fields, func(field string) bool {
		return len(field) == 0 || field == "." || field == ".."
	}) {
		return nil, fmt.Errorf("%w %q: expected %s", ErrInvalidKey, given, d.KeyUsage())
	}

	return fields, nil
}

// IDTemplate is the template of what identifies a row: ID, or the key
// fields joined by slashes.
func (d Definition) IDTemplate() string {
	if len(d.ID) > 0 {
		return d.ID
	}

	fields := make([]string, len(d.Key))
	for i, field := range d.Key {
		fields[i] = "{{." + field + "}}"
	}

	return strings.Join(fields, "/")
}

// KeyOf is the key of a row, as IDTemplate identifies it.
func (d Definition) KeyOf(row any) ([]string, error) {
	tmpl, err := template.New("id").Option("missingkey=zero").Parse(d.IDTemplate())
	if err != nil {
		return nil, fmt.Errorf("parsing the id of %s: %w", d.Plural, err)
	}

	var id strings.Builder
	if err := tmpl.Execute(&id, row); err != nil {
		return nil, fmt.Errorf("identifying the %s: %w", d.Name, err)
	}

	if strings.Contains(id.String(), noValue) {
		return nil, fmt.Errorf("%w: the %s has no %s", ErrInvalidKey, d.Name, d.KeyUsage())
	}

	return d.ParseKey(id.String())
}

// Rows are what a listing shows of the items the API answered it with: an
// item a row, but an item that keeps its versions (see Versions) a row for
// each of them, its own fields beside the version's.
func (d Definition) Rows(items []any) []any {
	if len(d.Versions) == 0 {
		return items
	}

	rows := make([]any, 0, len(items))

	for _, item := range items {
		object, isObject := item.(map[string]any)
		if !isObject {
			rows = append(rows, item)

			continue
		}

		versions, _ := object[d.Versions].([]any)
		for _, version := range versions {
			fields, isObject := version.(map[string]any)
			if !isObject {
				continue
			}

			row := make(map[string]any, len(object)+len(fields))
			maps.Copy(row, fields)

			for field, value := range object {
				if field != d.Versions {
					row[field] = value
				}
			}

			rows = append(rows, row)
		}
	}

	return rows
}

// definitions are every resource described, in the order they are shown.
var definitions = slices.Concat(content, learning, administration, account)

// All are every resource described, in the order they are shown.
func All() []Definition {
	return slices.Clone(definitions)
}

// Lookup is the resource of that name, singular or plural.
func Lookup(name string) (Definition, bool) {
	for _, d := range definitions {
		if d.Name == name || d.Plural == name {
			return d, true
		}
	}

	return Definition{}, false
}
