package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"text/template"

	"github.com/Tarhche/cli/internal/access"
	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// noValue is what a template writes of a field a row does not have, which
// a cell shows as nothing.
const noValue = "<no value>"

// Resources is how the frontend reaches what the dashboard manages beside
// the workload, each kind as package resource describes it: its generic
// pages are built from whatever definitions the session's permissions
// allow. Whose things are asked for is the frontend's to say: one's own,
// unless everybody's are asked for or one's own are not served, as the
// commands do. What needs a form of the web dashboard's, making something
// or changing it, is a link to that page of the context's website, as is
// what a row is shown as on it.
type Resources struct {
	workspace   *Workspace
	definitions func() []resource.Definition
}

// NewResources is the resource service of workspace, over the definitions
// definitions answers: resource.All.
func NewResources(workspace *Workspace, definitions func() []resource.Definition) *Resources {
	return &Resources{workspace: workspace, definitions: definitions}
}

// ResourceDefinition is a kind of resource as the frontend builds its pages
// from it.
type ResourceDefinition struct {
	Name        string `json:"name"`
	Plural      string `json:"plural"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Section     string `json:"section"`

	// Key are the fields that identify one, in order.
	Key []string `json:"key"`

	// Audiences are whose things the session may list of it, one's own
	// (mine) first, then everybody's (everyone).
	Audiences []string `json:"audiences"`

	// Mine are what the session may ask of one's own, and Everyone what it
	// may ask of everybody's.
	Mine     Routes `json:"mine"`
	Everyone Routes `json:"everyone"`

	Actions []ResourceAction `json:"actions"`

	// Filters are what its listings can be narrowed by.
	Filters []ResourceFilter `json:"filters"`

	// Cursor says its listing is paged by a cursor, the next a page names,
	// rather than by number.
	Cursor bool `json:"cursor"`

	// Columns are the headers of its listing's table.
	Columns []string `json:"columns"`

	// New is where one is made, on the web dashboard: nothing when the
	// session may make none, or its context names no website.
	New ResourceNew `json:"new"`

	// Links are where a row is shown or changed beside its listing, in the
	// order they are offered.
	Links []ResourceLink `json:"links"`
}

// ResourceNew is the page of the web dashboard one of a resource is made
// on.
type ResourceNew struct {
	// Name is what it is offered as: "New article".
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ResourceLink is a page a row of a resource is shown or changed on: one of
// the context's website or API, which a row's link is the address of, or
// one of the app, which a row's link is the value of a filter of.
type ResourceLink struct {
	// Name is what it is offered as: "Edit article".
	Name string `json:"name"`

	// Kind is what it does: view, edit, list or public.
	Kind string `json:"kind"`

	// Resource is, for a page of the app, the resource whose everybody's
	// listing it opens, and Filter the filter that listing is narrowed by;
	// both empty for a page of a site.
	Resource string `json:"resource"`
	Filter   string `json:"filter"`

	// Audiences are the listings the session is offered it in: mine,
	// everyone.
	Audiences []string `json:"audiences"`
}

// Routes say what the session may ask an audience of a resource: what is
// served there that its permissions allow.
type Routes struct {
	List   bool `json:"list"`
	Get    bool `json:"get"`
	Create bool `json:"create"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

// ResourceAction is something more a resource can be asked.
type ResourceAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Title is what it is offered as: "Mark as read".
	Title string `json:"title"`

	// Confirm is what is asked before it is, nothing for nothing.
	Confirm string `json:"confirm"`

	// Mine says it is asked of one's own, else of everybody's.
	Mine bool `json:"mine"`

	// Keyed says it is asked of one thing, by its key, rather than of the
	// resource.
	Keyed bool `json:"keyed"`

	// Allowed says the session's permissions allow it.
	Allowed bool `json:"allowed"`
}

// ResourceFilter is what a listing can be narrowed by: a query parameter.
type ResourceFilter struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Values are what it takes, when it takes only some.
	Values []string `json:"values"`

	// Moment says it takes a moment, in RFC 3339.
	Moment bool `json:"moment"`

	// Audiences are the listings that take it, mine or everyone; none is
	// every one.
	Audiences []string `json:"audiences"`
}

// ListQuery says which page of a listing to read, and what narrows it.
type ListQuery struct {
	// Page is the page of a listing paged by number; the first is 1.
	Page uint `json:"page"`

	// Cursor is where a listing paged by a cursor goes on from: the next
	// its last page named, empty for its first page.
	Cursor string `json:"cursor"`

	// Filters are the values of its filters, by name; an empty one
	// narrows nothing.
	Filters map[string]string `json:"filters"`
}

// Listing is a page of a resource's listing, laid out as its table.
type Listing struct {
	Columns     []string `json:"columns"`
	Rows        []Row    `json:"rows"`
	TotalPages  uint     `json:"total_pages"`
	CurrentPage uint     `json:"current_page"`

	// Next is where a listing paged by a cursor goes on from, empty once it
	// is all read.
	Next string `json:"next"`
}

// Row is one row of a listing.
type Row struct {
	// Key is what identifies it, in the order of its definition's key,
	// empty when it says none.
	Key []string `json:"key"`

	// Cells are its columns, as its definition's table writes them.
	Cells []string `json:"cells"`

	// Item is the row as the API answered it.
	Item json.RawMessage `json:"item"`

	// Links are where its definition's links lead for it, by name: an
	// address, or, for a page of the app, the value of its filter. A link
	// it has none of is not offered for it.
	Links map[string]string `json:"links"`

	// Actions are the names of its definition's actions of one thing that
	// are offered for it.
	Actions []string `json:"actions"`

	// Protected says it is not deleted: the default role.
	Protected bool `json:"protected"`
}

// Definitions are the kinds of resource the session may list, in the order
// they are shown, each with what its permissions allow of it: a hint of
// what to offer, as the API decides every request.
func (r *Resources) Definitions(ctx context.Context) ([]ResourceDefinition, error) {
	granted, err := r.workspace.Granted(ctx)
	if err != nil {
		return nil, err
	}

	c, err := r.workspace.Context()
	if err != nil {
		return nil, err
	}

	described := []ResourceDefinition{}

	for _, d := range r.definitions() {
		if offered := describe(d, granted, sitesOf(c)); len(offered.Audiences) > 0 {
			described = append(described, offered)
		}
	}

	return withPagesListed(described), nil
}

// List reads a page of a resource's listing.
func (r *Resources) List(ctx context.Context, name string, everyone bool, query ListQuery) (Listing, error) {
	d, err := r.lookup(name)
	if err != nil {
		return Listing{}, err
	}

	path := routeOf(d, operationList, everyone)
	if len(path) == 0 {
		return Listing{}, unserved(d, "listed")
	}

	layout, err := layoutOf(d, audienceOf(everyone))
	if err != nil {
		return Listing{}, err
	}

	c, err := r.workspace.Context()
	if err != nil {
		return Listing{}, err
	}

	client, err := r.workspace.Client(ctx)
	if err != nil {
		return Listing{}, err
	}

	listing, raws, err := read(ctx, client, path, d.Cursor, query)
	if err != nil {
		return Listing{}, err
	}

	rows := d.Rows(decodeItems(raws))
	listing.Columns = headersOf(d)
	listing.Rows = make([]Row, 0, len(rows))

	for _, row := range rows {
		laidOut, err := layout.row(d, sitesOf(c), row)
		if err != nil {
			return Listing{}, err
		}

		listing.Rows = append(listing.Rows, laidOut)
	}

	return withoutEmptyColumns(listing), nil
}

// Delete deletes one of a resource, by its key.
func (r *Resources) Delete(ctx context.Context, name string, everyone bool, key []string) error {
	d, err := r.lookup(name)
	if err != nil {
		return err
	}

	path := routeOf(d, operationDelete, everyone)
	if len(path) == 0 {
		return unserved(d, "deleted")
	}

	body := d.BodyKey(path, key)
	if len(body) > 0 {
		body = withDefaults(body, d.Defaults)
	}

	_, err = r.send(ctx, methods[operationDelete], d.Path(path, key), requestBody(body))

	return err
}

// Action asks a resource something more than reading, creating, changing
// and deleting: one thing of it, by its key, when the action is keyed.
func (r *Resources) Action(ctx context.Context, name, action string, everyone bool, key []string) (json.RawMessage, error) {
	d, err := r.lookup(name)
	if err != nil {
		return nil, err
	}

	a, found := chosenAction(d, action, everyone)
	if !found {
		return nil, fmt.Errorf("%s cannot be asked to %s", d.Title, action)
	}

	method := methodOf(a)

	if !keyed(d, a) {
		return r.send(ctx, method, a.Path, requestBody(a.Body))
	}

	if len(key) != len(d.Key) {
		return nil, fmt.Errorf("say which %s to %s", d.Name, action)
	}

	body := cloned(a.Body)
	maps.Copy(body, d.BodyKey(a.Path, key))

	return r.send(ctx, method, d.Path(a.Path, key), requestBody(body))
}

func (r *Resources) lookup(name string) (resource.Definition, error) {
	for _, d := range r.definitions() {
		if d.Name == name || d.Plural == name {
			return d, nil
		}
	}

	return resource.Definition{}, fmt.Errorf("there is no resource called %q", name)
}

func (r *Resources) send(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	client, err := r.workspace.Client(ctx)
	if err != nil {
		return nil, err
	}

	var answer json.RawMessage
	if err := client.Do(ctx, method, path, nil, body, &answer); err != nil {
		return nil, err
	}

	return answered(answer), nil
}

// read reads a page of the listing at path: by its cursor, for a listing
// paged by one, which cursor names the query parameter of, or by its number.
func read(ctx context.Context, client *api.Client, path, cursor string, query ListQuery) (Listing, []json.RawMessage, error) {
	values := url.Values{}
	for name, value := range query.Filters {
		if len(value) > 0 {
			values.Set(name, value)
		}
	}

	if len(cursor) > 0 {
		if len(query.Cursor) > 0 {
			values.Set(cursor, query.Cursor)
		}

		var page struct {
			Items []json.RawMessage `json:"items"`
			Next  string            `json:"next"`
		}

		if err := client.Get(ctx, path, values, &page); err != nil {
			return Listing{}, nil, err
		}

		return Listing{Next: page.Next}, page.Items, nil
	}

	page, err := api.List[json.RawMessage](ctx, client, path, values, query.Page)
	if err != nil {
		return Listing{}, nil, err
	}

	return Listing{TotalPages: page.Pagination.TotalPages, CurrentPage: page.Pagination.CurrentPage}, page.Items, nil
}

// describe is a resource as the frontend builds its pages from it, for a
// session granted what granted says, in a context whose sites are sites:
// the actions the app offers, and the links it can follow.
func describe(d resource.Definition, granted access.Granted, sites sites) ResourceDefinition {
	actions := []ResourceAction{}

	for _, a := range d.Actions {
		if len(a.Title) > 0 {
			actions = append(actions, ResourceAction{
				Name: a.Name, Description: a.Description, Title: a.Title, Confirm: a.Confirm,
				Mine: a.Mine, Keyed: keyed(d, a), Allowed: granted.Allows(methodOf(a), a.Path),
			})
		}
	}

	audiences := audiencesOf(d, granted)
	mine, everyone := routesOf(d, false, granted), routesOf(d, true, granted)

	return ResourceDefinition{
		Name:        d.Name,
		Plural:      d.Plural,
		Title:       d.Title,
		Description: d.Description,
		Section:     d.Section,
		Key:         append([]string{}, d.Key...),
		Audiences:   audiences,
		Mine:        mine,
		Everyone:    everyone,
		Actions:     actions,
		Filters:     filtersOf(d),
		Cursor:      len(d.Cursor) > 0,
		Columns:     headersOf(d),
		New:         newOf(d, mine.Create || everyone.Create, sites),
		Links:       linksOf(d, audiences, granted, sites),
	}
}

// routesOf are what a session granted what granted says may ask an
// audience of a resource, a route of the other audience serving what its
// own does not.
func routesOf(d resource.Definition, everyone bool, granted access.Granted) Routes {
	allowed := func(op operation) bool {
		path := routeOf(d, op, everyone)

		return len(path) > 0 && granted.Allows(methods[op], path)
	}

	return Routes{
		List:   allowed(operationList),
		Get:    allowed(operationGet),
		Create: allowed(operationCreate),
		Update: allowed(operationUpdate),
		Delete: allowed(operationDelete),
	}
}

// table is a resource's table in the app, one template per column separated
// by tabs: its definition's, else its key.
func table(d resource.Definition) string {
	format := strings.ReplaceAll(d.AppTable(), `\t`, "\t")
	if len(strings.TrimSpace(format)) > 0 {
		return format
	}

	columns := make([]string, len(d.Key))
	for i, field := range d.Key {
		columns[i] = "{{." + field + "}}"
	}

	return strings.Join(columns, "\t")
}

// headersOf are the headers of a resource's table, as a sentence begins:
// "Passing score".
func headersOf(d resource.Definition) []string {
	headers := strings.Split(formatter.Header(table(d), d.Headers), "\t")

	for i, header := range headers {
		headers[i] = sentence(header)
	}

	return headers
}

// sentence is a header in capitals as a sentence begins: its first letter
// a capital, the rest small.
func sentence(header string) string {
	lower := []rune(strings.ToLower(header))
	if len(lower) == 0 {
		return header
	}

	return strings.ToUpper(string(lower[:1])) + string(lower[1:])
}

// cell writes one column of a listing's rows.
type cell struct {
	template *template.Template
}

// write is the column of row, with nothing where the row has no field.
func (c cell) write(row any) (string, error) {
	var written bytes.Buffer
	if err := c.template.Execute(&written, row); err != nil {
		return "", err
	}

	return strings.ReplaceAll(written.String(), noValue, ""), nil
}

// answered is what the API answered, none when it answered nothing.
func answered(answer json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(answer)) == 0 {
		return nil
	}

	return answer
}

func unserved(d resource.Definition, done string) error {
	return fmt.Errorf("%s cannot be %s here", d.Title, done)
}
