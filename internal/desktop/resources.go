package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// noValue is what a template writes of a field a row does not have, which
// a cell shows as nothing.
const noValue = "<no value>"

// Resources is how the frontend reaches what the dashboard manages beside
// the workload, each kind as package resource describes it: its generic
// pages are built from whatever definitions there are. Whose things are
// asked for is the frontend's to say: one's own, unless everybody's are
// asked for or one's own are not served, as the commands do.
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

	// Audiences are whose things it is listed of, one's own (mine) first,
	// then everybody's (everyone).
	Audiences []string `json:"audiences"`

	// Mine are what one's own can be asked, and Everyone what everybody's
	// can.
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
}

// Routes say what an audience of a resource can be asked.
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

	// Mine says it is asked of one's own, else of everybody's.
	Mine bool `json:"mine"`

	// Keyed says it is asked of one thing, by its key, rather than of the
	// resource.
	Keyed bool `json:"keyed"`
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
}

// Definitions are every kind of resource there is, in the order they are
// shown.
func (r *Resources) Definitions() []ResourceDefinition {
	definitions := r.definitions()
	described := make([]ResourceDefinition, len(definitions))

	for i, d := range definitions {
		described[i] = describe(d)
	}

	return described
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

	cells, err := cellsOf(d)
	if err != nil {
		return Listing{}, err
	}

	client, err := r.workspace.Client(ctx)
	if err != nil {
		return Listing{}, err
	}

	listing, raws, err := read(ctx, client, path, cursorOf(d), query)
	if err != nil {
		return Listing{}, err
	}

	rows := rowsOf(d, decodeItems(raws))
	listing.Columns = headersOf(d)
	listing.Rows = make([]Row, 0, len(rows))

	for _, row := range rows {
		laidOut, err := rowOf(d, cells, row)
		if err != nil {
			return Listing{}, err
		}

		listing.Rows = append(listing.Rows, laidOut)
	}

	return listing, nil
}

// Get reads one of a resource, by its key.
func (r *Resources) Get(ctx context.Context, name string, everyone bool, key []string) (json.RawMessage, error) {
	d, err := r.lookup(name)
	if err != nil {
		return nil, err
	}

	path := routeOf(d, operationGet, everyone)
	if len(path) == 0 {
		return nil, unserved(d, "read")
	}

	return r.send(ctx, http.MethodGet, d.Path(path, key), nil)
}

// Create creates one of a resource from body, its JSON object.
func (r *Resources) Create(ctx context.Context, name string, everyone bool, body string) (json.RawMessage, error) {
	d, err := r.lookup(name)
	if err != nil {
		return nil, err
	}

	path := routeOf(d, operationCreate, everyone)
	if len(path) == 0 {
		return nil, unserved(d, "created")
	}

	object, err := decodeObject(body)
	if err != nil {
		return nil, err
	}

	return r.send(ctx, http.MethodPost, path, withDefaults(object, defaultsOf(d)))
}

// Update changes one of a resource to body, its JSON object: the one key
// names, or, given no key, the one body says.
func (r *Resources) Update(ctx context.Context, name string, everyone bool, key []string, body string) (json.RawMessage, error) {
	d, err := r.lookup(name)
	if err != nil {
		return nil, err
	}

	path := routeOf(d, operationUpdate, everyone)
	if len(path) == 0 {
		return nil, unserved(d, "changed")
	}

	object, err := decodeObject(body)
	if err != nil {
		return nil, err
	}

	if len(key) == 0 {
		if key, err = keyFromBody(d, object); err != nil {
			return nil, err
		}
	}

	maps.Copy(object, bodyKeyOf(d, path, key))

	return r.send(ctx, http.MethodPut, d.Path(path, key), withDefaults(object, defaultsOf(d)))
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

	body := bodyKeyOf(d, path, key)
	if len(body) > 0 {
		body = withDefaults(body, defaultsOf(d))
	}

	_, err = r.send(ctx, http.MethodDelete, d.Path(path, key), requestBody(body))

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

	method := a.Method
	if len(method) == 0 {
		method = http.MethodPost
	}

	if !keyed(d, a) {
		return r.send(ctx, method, a.Path, requestBody(actionBodyOf(a)))
	}

	if len(key) != len(d.Key) {
		return nil, fmt.Errorf("say which %s to %s", d.Name, action)
	}

	body := cloned(actionBodyOf(a))
	maps.Copy(body, bodyKeyOf(d, a.Path, key))

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

func describe(d resource.Definition) ResourceDefinition {
	actions := make([]ResourceAction, len(d.Actions))
	for i, a := range d.Actions {
		actions[i] = ResourceAction{Name: a.Name, Description: a.Description, Mine: a.Mine, Keyed: keyed(d, a)}
	}

	return ResourceDefinition{
		Name:        d.Name,
		Plural:      d.Plural,
		Title:       d.Title,
		Description: d.Description,
		Section:     d.Section,
		Key:         append([]string{}, d.Key...),
		Audiences:   audiencesOf(d),
		Mine:        routesOf(d, false),
		Everyone:    routesOf(d, true),
		Actions:     actions,
		Filters:     filtersOf(d),
		Cursor:      len(cursorOf(d)) > 0,
		Columns:     headersOf(d),
	}
}

// routesOf are what an audience of a resource can be asked, a route of the
// other audience serving what its own does not.
func routesOf(d resource.Definition, everyone bool) Routes {
	served := func(op operation) bool { return len(routeOf(d, op, everyone)) > 0 }

	return Routes{
		List:   served(operationList),
		Get:    served(operationGet),
		Create: served(operationCreate),
		Update: served(operationUpdate),
		Delete: served(operationDelete),
	}
}

// table is a resource's table, one template per column separated by tabs:
// its definition's, else its key.
func table(d resource.Definition) string {
	format := strings.ReplaceAll(d.Table, `\t`, "\t")
	if len(strings.TrimSpace(format)) > 0 {
		return format
	}

	columns := make([]string, len(d.Key))
	for i, field := range d.Key {
		columns[i] = "{{." + field + "}}"
	}

	return strings.Join(columns, "\t")
}

// headersOf are the headers of a resource's table.
func headersOf(d resource.Definition) []string {
	return strings.Split(formatter.Header(table(d), d.Headers), "\t")
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

func cellsOf(d resource.Definition) ([]cell, error) {
	formats := strings.Split(table(d), "\t")
	cells := make([]cell, len(formats))

	for i, format := range formats {
		parsed, err := formatter.Parse(format)
		if err != nil {
			return nil, err
		}

		cells[i] = cell{template: parsed}
	}

	return cells, nil
}

// rowOf is a row of a listing as it is laid out: its key, which a row that
// says none has none of, its cells, and itself, which, read from JSON, is
// always written as JSON again.
func rowOf(d resource.Definition, cells []cell, row any) (Row, error) {
	key, err := keyOfRow(d, row)
	if err != nil {
		key = []string{}
	}

	item, _ := json.Marshal(row)
	laidOut := Row{Key: key, Cells: make([]string, len(cells)), Item: item}

	for i, c := range cells {
		if laidOut.Cells[i], err = c.write(row); err != nil {
			return Row{}, err
		}
	}

	return laidOut, nil
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
