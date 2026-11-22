package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/resource"
)

// noValue is what a template writes of a field an item does not have,
// which a cell shows as nothing.
const noValue = "<no value>"

// Resources is how the frontend reaches what the dashboard manages beside
// the workload, each kind as package resource describes it: its generic
// pages are built from whatever definitions there are.
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

	// Everyone are the routes of anybody's, and Mine of one's own.
	Everyone Routes `json:"everyone"`
	Mine     Routes `json:"mine"`

	Actions []ResourceAction `json:"actions"`

	// Columns are the headers of its listing's table.
	Columns []string `json:"columns"`
}

// Routes say which of a resource's routes an audience has.
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

	// Mine says it is asked of one's own, else of anybody's.
	Mine bool `json:"mine"`
}

// Listing is a page of a resource's listing, laid out as its table.
type Listing struct {
	Columns     []string `json:"columns"`
	Rows        []Row    `json:"rows"`
	TotalPages  uint     `json:"total_pages"`
	CurrentPage uint     `json:"current_page"`
}

// Row is one item of a listing.
type Row struct {
	// Key is what identifies it, in the order of its definition's key.
	Key []string `json:"key"`

	// Cells are its columns, as its definition's table writes them.
	Cells []string `json:"cells"`

	// Item is the item as the API answered it.
	Item json.RawMessage `json:"item"`
}

// Definitions are every kind of resource there is, in the order they are
// shown.
func (r *Resources) Definitions() []ResourceDefinition {
	definitions := r.definitions()
	described := make([]ResourceDefinition, len(definitions))

	for i, d := range definitions {
		described[i] = ResourceDefinition{
			Name:        d.Name,
			Plural:      d.Plural,
			Title:       d.Title,
			Description: d.Description,
			Section:     d.Section,
			Key:         append([]string{}, d.Key...),
			Everyone:    routesOf(d.Everyone),
			Mine:        routesOf(d.Mine),
			Actions:     actionsOf(d.Actions),
			Columns:     headersOf(d),
		}
	}

	return described
}

// List reads a page of a resource's listing, one's own when mine and
// anybody's otherwise; the first page is 1.
func (r *Resources) List(ctx context.Context, name string, mine bool, page uint) (Listing, error) {
	d, paths, err := r.lookup(name, mine)
	if err != nil {
		return Listing{}, err
	}

	if len(paths.List) == 0 {
		return Listing{}, unserved(d, "listed", mine)
	}

	client, err := r.workspace.Client(ctx)
	if err != nil {
		return Listing{}, err
	}

	answered, err := api.List[json.RawMessage](ctx, client, paths.List, nil, page)
	if err != nil {
		return Listing{}, err
	}

	cells, err := cellsOf(d)
	if err != nil {
		return Listing{}, err
	}

	listing := Listing{
		Columns:     headersOf(d),
		Rows:        make([]Row, 0, len(answered.Items)),
		TotalPages:  answered.Pagination.TotalPages,
		CurrentPage: answered.Pagination.CurrentPage,
	}

	for _, item := range answered.Items {
		row, err := rowOf(d, cells, item)
		if err != nil {
			return Listing{}, err
		}

		listing.Rows = append(listing.Rows, row)
	}

	return listing, nil
}

// Get reads one of a resource, by its key.
func (r *Resources) Get(ctx context.Context, name string, mine bool, key []string) (json.RawMessage, error) {
	d, paths, err := r.lookup(name, mine)
	if err != nil {
		return nil, err
	}

	if len(paths.Get) == 0 {
		return nil, unserved(d, "read", mine)
	}

	return r.send(ctx, http.MethodGet, d.Path(paths.Get, key), nil)
}

// Create creates one of a resource from body, its JSON.
func (r *Resources) Create(ctx context.Context, name string, mine bool, body string) (json.RawMessage, error) {
	d, paths, err := r.lookup(name, mine)
	if err != nil {
		return nil, err
	}

	if len(paths.Create) == 0 {
		return nil, unserved(d, "created", mine)
	}

	if err := validJSON(body); err != nil {
		return nil, err
	}

	return r.send(ctx, http.MethodPost, paths.Create, json.RawMessage(body))
}

// Update changes one of a resource to body, its JSON, which carries its key.
func (r *Resources) Update(ctx context.Context, name string, mine bool, body string) (json.RawMessage, error) {
	d, paths, err := r.lookup(name, mine)
	if err != nil {
		return nil, err
	}

	if len(paths.Update) == 0 {
		return nil, unserved(d, "changed", mine)
	}

	if err := validJSON(body); err != nil {
		return nil, err
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return nil, InvalidError{Fields: map[string]string{"json": "it has to be an object"}}
	}

	return r.send(ctx, http.MethodPut, d.Path(paths.Update, keyOf(d, fields)), json.RawMessage(body))
}

// Delete deletes one of a resource, by its key.
func (r *Resources) Delete(ctx context.Context, name string, mine bool, key []string) error {
	d, paths, err := r.lookup(name, mine)
	if err != nil {
		return err
	}

	if len(paths.Delete) == 0 {
		return unserved(d, "deleted", mine)
	}

	_, err = r.send(ctx, http.MethodDelete, d.Path(paths.Delete, key), nil)

	return err
}

// Action asks one of a resource, by its key, something more than reading,
// creating, changing and deleting.
func (r *Resources) Action(ctx context.Context, name, action string, key []string) (json.RawMessage, error) {
	d, found := resourceNamed(r.definitions(), name)
	if !found {
		return nil, fmt.Errorf("there is no resource called %q", name)
	}

	for _, a := range d.Actions {
		if a.Name == action {
			method := a.Method
			if len(method) == 0 {
				method = http.MethodPost
			}

			return r.send(ctx, method, d.Path(a.Path, key), nil)
		}
	}

	return nil, fmt.Errorf("%s cannot be asked to %s", d.Title, action)
}

func (r *Resources) lookup(name string, mine bool) (resource.Definition, resource.Paths, error) {
	d, found := resourceNamed(r.definitions(), name)
	if !found {
		return resource.Definition{}, resource.Paths{}, fmt.Errorf("there is no resource called %q", name)
	}

	if mine {
		return d, d.Mine, nil
	}

	return d, d.Everyone, nil
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

func resourceNamed(definitions []resource.Definition, name string) (resource.Definition, bool) {
	for _, d := range definitions {
		if d.Name == name || d.Plural == name {
			return d, true
		}
	}

	return resource.Definition{}, false
}

func routesOf(p resource.Paths) Routes {
	return Routes{
		List:   len(p.List) > 0,
		Get:    len(p.Get) > 0,
		Create: len(p.Create) > 0,
		Update: len(p.Update) > 0,
		Delete: len(p.Delete) > 0,
	}
}

func actionsOf(actions []resource.Action) []ResourceAction {
	described := make([]ResourceAction, len(actions))
	for i, a := range actions {
		described[i] = ResourceAction{Name: a.Name, Description: a.Description, Mine: a.Mine}
	}

	return described
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

// cell writes one column of a listing's rows.
type cell struct {
	template *template.Template
}

// write is the column of item, with nothing where the item has no field.
func (c cell) write(item any) (string, error) {
	var written bytes.Buffer
	if err := c.template.Execute(&written, item); err != nil {
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

// headersOf are the headers of a resource's table.
func headersOf(d resource.Definition) []string {
	return strings.Split(formatter.Header(table(d), d.Headers), "\t")
}

// rowOf is an item of a listing as its row. The item is JSON, since the
// page it is on was read whole.
func rowOf(d resource.Definition, cells []cell, item json.RawMessage) (Row, error) {
	var decoded any
	_ = json.Unmarshal(item, &decoded)

	fields, _ := decoded.(map[string]any)
	row := Row{Key: keyOf(d, fields), Cells: make([]string, len(cells)), Item: item}

	for i, c := range cells {
		written, err := c.write(decoded)
		if err != nil {
			return Row{}, err
		}

		row.Cells[i] = written
	}

	return row, nil
}

// keyOf is what identifies an item, from its fields, in the order of its
// definition's key: a field it does not have is empty.
func keyOf(d resource.Definition, fields map[string]any) []string {
	key := make([]string, len(d.Key))

	for i, field := range d.Key {
		if value, found := fields[field]; found && value != nil {
			key[i] = fmt.Sprint(value)
		}
	}

	return key
}

func validJSON(body string) error {
	if !json.Valid([]byte(body)) {
		return InvalidError{Fields: map[string]string{"json": "it is not valid JSON"}}
	}

	return nil
}

// answered is what the API answered, none when it answered nothing.
func answered(answer json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(answer)) == 0 {
		return nil
	}

	return answer
}

func unserved(d resource.Definition, done string, mine bool) error {
	whose := "anybody's"
	if mine {
		whose = "one's own"
	}

	return fmt.Errorf("%s of %s cannot be %s here", d.Title, whose, done)
}
