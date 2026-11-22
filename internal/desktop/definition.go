package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/Tarhche/cli/internal/resource"
)

// Audiences of a resource: whose things a route serves.
const (
	audienceMine     = "mine"
	audienceEveryone = "everyone"
)

// operation is one of the five things a resource may be asked.
type operation string

const (
	operationList   operation = "list"
	operationGet    operation = "get"
	operationCreate operation = "create"
	operationUpdate operation = "update"
	operationDelete operation = "delete"
)

// pathOf is the path of paths op is served at, empty when it is not served.
func pathOf(paths resource.Paths, op operation) string {
	switch op {
	case operationList:
		return paths.List
	case operationGet:
		return paths.Get
	case operationCreate:
		return paths.Create
	case operationUpdate:
		return paths.Update
	default:
		return paths.Delete
	}
}

// routeOf is the path op is served at: for one's own things unless
// everybody's are asked for or one's own are not served, for everybody's
// otherwise; empty when op is not served that way.
func routeOf(d resource.Definition, op operation, everyone bool) string {
	if !everyone {
		if path := pathOf(d.Mine, op); len(path) > 0 {
			return path
		}
	}

	return pathOf(d.Everyone, op)
}

// audiencesOf are the audiences a resource is listed to, one's own first.
func audiencesOf(d resource.Definition) []string {
	audiences := []string{}

	if len(d.Mine.List) > 0 {
		audiences = append(audiences, audienceMine)
	}

	if len(d.Everyone.List) > 0 {
		audiences = append(audiences, audienceEveryone)
	}

	return audiences
}

// bodyKeyOf are the fields of key that path does not name in braces, by
// field: what a request to it carries in its body instead.
func bodyKeyOf(d resource.Definition, path string, key []string) map[string]any {
	fields := make(map[string]any)

	for i, field := range d.Key {
		if i < len(key) && !strings.Contains(path, "{"+field+"}") {
			fields[field] = key[i]
		}
	}

	return fields
}

// keyed reports whether an action is asked of one thing of its resource,
// which its route names the key of, rather than of the resource.
func keyed(d resource.Definition, a resource.Action) bool {
	return len(bodyKeyOf(d, a.Path, d.Key)) < len(d.Key)
}

// rowsOf are what a listing shows of the items the API answered it with:
// an item a row.
func rowsOf(_ resource.Definition, items []any) []any {
	return items
}

// keyOfRow is the key of a row: its key fields, in order.
func keyOfRow(d resource.Definition, row any) ([]string, error) {
	fields, _ := row.(map[string]any)
	key := make([]string, len(d.Key))

	for i, field := range d.Key {
		value, found := fields[field]
		if !found || value == nil || fmt.Sprint(value) == "" {
			return nil, fmt.Errorf("the %s has no %s", d.Name, field)
		}

		key[i] = fmt.Sprint(value)
	}

	return key, nil
}

// defaultsOf are the fields a body sent to a resource carries when it does
// not say them itself.
func defaultsOf(resource.Definition) map[string]any {
	return nil
}

// actionBodyOf is what an action is sent with, nil for nothing.
func actionBodyOf(resource.Action) map[string]any {
	return nil
}

// filtersOf are the filters the listings of a resource take.
func filtersOf(resource.Definition) []ResourceFilter {
	return []ResourceFilter{}
}

// cursorOf is the query parameter a listing paged by a cursor is asked for
// its next page with, empty for a listing paged by number.
func cursorOf(resource.Definition) string {
	return ""
}

// chosenAction is the action of that name asked of the audience asked, or
// of the other when only it is served.
func chosenAction(d resource.Definition, name string, everyone bool) (resource.Action, bool) {
	var other *resource.Action

	for i, a := range d.Actions {
		if a.Name != name {
			continue
		}

		if a.Mine != everyone {
			return a, true
		}

		if other == nil {
			other = &d.Actions[i]
		}
	}

	if other == nil {
		return resource.Action{}, false
	}

	return *other, true
}

// withDefaults is body with the defaults it does not say itself.
func withDefaults(body, defaults map[string]any) map[string]any {
	for field, value := range defaults {
		if _, said := body[field]; !said {
			body[field] = value
		}
	}

	return body
}

// requestBody is what a request carries of fields: nothing for none.
func requestBody(fields map[string]any) any {
	if len(fields) == 0 {
		return nil
	}

	return fields
}

// decodeJSON is JSON content as a value, its numbers as they were written,
// so that a template writes 31536000 rather than 3.1536e+07; nothing at
// all is null.
func decodeJSON(content []byte) (any, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	return value, nil
}

// decodeItems are the items of a page as decodeJSON reads each: they are
// JSON, since the page they are on was read whole.
func decodeItems(raws []json.RawMessage) []any {
	items := make([]any, len(raws))
	for i, raw := range raws {
		items[i], _ = decodeJSON(raw)
	}

	return items
}

// decodeObject is the JSON object body holds, refused field by field when
// it is none: an array of one object, as inspect writes it, holds that
// object.
func decodeObject(body string) (map[string]any, error) {
	value, err := decodeJSON([]byte(body))
	if err != nil {
		return nil, InvalidError{Fields: map[string]string{"json": "it is not valid JSON"}}
	}

	if array, isArray := value.([]any); isArray && len(array) == 1 {
		value = array[0]
	}

	object, isObject := value.(map[string]any)
	if !isObject {
		return nil, InvalidError{Fields: map[string]string{"json": "it has to be an object"}}
	}

	return object, nil
}

// keyFromBody is the key a body says itself, which an update carries when
// it is given none.
func keyFromBody(d resource.Definition, body map[string]any) ([]string, error) {
	key := make([]string, len(d.Key))

	for i, field := range d.Key {
		value, isString := body[field].(string)
		if !isString || len(value) == 0 {
			return nil, InvalidError{Fields: map[string]string{field: "say which " + d.Name + " it is: its " + field}}
		}

		key[i] = value
	}

	return key, nil
}

// cloned is fields, which may be nil, as a map of its own.
func cloned(fields map[string]any) map[string]any {
	if fields == nil {
		return make(map[string]any)
	}

	return maps.Clone(fields)
}
